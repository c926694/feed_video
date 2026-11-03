package message

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	commentevent "simple_tiktok/internal/modules/comment/event"
	commentrepo "simple_tiktok/internal/modules/comment/repo"
	followevent "simple_tiktok/internal/modules/follow/event"
	likeevent "simple_tiktok/internal/modules/like/event"
	messagerepo "simple_tiktok/internal/modules/message/repo"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/upload"
)

const (
	defaultListLimit = 20
	maxListLimit     = 50

	// 会话窗口：8 小时一个桶，同一个桶内同目标的互动累加到同一行
	windowSeconds = int64(8 * 60 * 60)

	// 评论摘要的长度上限
	contentMaxRunes = 80
)

// Logic 通知模块的业务逻辑。列表要显示触发者与目标封面，
// 所以用到 user 与 video 两个模块的仓储
type Logic struct {
	repo     *messagerepo.Repo
	users    *userrepo.Repo
	videos   *videorepo.Repo
	comments *commentrepo.Repo
	uploader *upload.Uploader
}

// NormalizeListLimit 归一化列表条数
func NormalizeListLimit(limit uint64) uint64 {
	if limit == 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

// HandleLikeSwitched 点赞事件产生通知。取消点赞不通知，自己给自己点赞不通知
func (l *Logic) HandleLikeSwitched(ctx context.Context, payload []byte) error {
	var switched likeevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if !switched.Liked {
		return nil
	}
	if switched.Operator == 0 || switched.TargetID == 0 {
		return consumer.Permanent(errors.New("点赞事件里缺少用户或目标 ID"))
	}
	occurredAt := millisToTime(switched.OccurredAt)

	var input NotifyInput
	var ok bool
	var err error
	switch switched.Target {
	case likeevent.TargetVideo:
		input, ok, err = l.resolveVideoLike(ctx, switched.TargetID, switched.Operator, occurredAt)
	case likeevent.TargetComment:
		input, ok, err = l.resolveCommentLike(ctx, switched.TargetID, switched.Operator, occurredAt)
	default:
		return nil
	}
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	_, err = l.Notify(ctx, input)
	return err
}

// resolveVideoLike 点赞视频产生的通知，接收者是视频作者
func (l *Logic) resolveVideoLike(ctx context.Context, videoID uint64, operator uint64, at time.Time) (NotifyInput, bool, error) {
	video, err := l.videos.GetByID(ctx, videoID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotifyInput{}, false, nil
	}
	if err != nil {
		return NotifyInput{}, false, err
	}
	if video.AuthorID == 0 || video.AuthorID == operator {
		return NotifyInput{}, false, nil
	}
	return NotifyInput{
		UserID:     video.AuthorID,
		Type:       messagerepo.TypeLikeVideo,
		TargetID:   video.ID,
		ActorID:    operator,
		VideoID:    video.ID,
		OccurredAt: at,
	}, true, nil
}

// resolveCommentLike 点赞评论产生的通知，接收者是评论作者
func (l *Logic) resolveCommentLike(ctx context.Context, commentID uint64, operator uint64, at time.Time) (NotifyInput, bool, error) {
	comment, err := l.comments.GetByID(ctx, commentID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotifyInput{}, false, nil
	}
	if err != nil {
		return NotifyInput{}, false, err
	}
	if comment.Commenter == 0 || comment.Commenter == operator {
		return NotifyInput{}, false, nil
	}
	return NotifyInput{
		UserID:     comment.Commenter,
		Type:       messagerepo.TypeLikeComment,
		TargetID:   comment.ID,
		ActorID:    operator,
		VideoID:    comment.VideoID,
		OccurredAt: at,
	}, true, nil
}

// HandleCommentCreated 评论事件产生通知：回复发给被回复的人，顶级评论发给视频作者。
// 事件里没有评论正文与被回复者，所以读一次评论行
func (l *Logic) HandleCommentCreated(ctx context.Context, payload []byte) error {
	var created commentevent.CreatedEvent
	if err := json.Unmarshal(payload, &created); err != nil {
		return consumer.Permanent(err)
	}
	if created.CommentID == 0 {
		return consumer.Permanent(errors.New("评论事件里没有评论 ID"))
	}
	input, ok, err := l.resolveComment(ctx, created.CommentID, millisToTime(created.OccurredAt))
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	_, err = l.Notify(ctx, input)
	return err
}

// resolveComment 评论产生的通知：回复发给被回复的人，顶级评论发给视频作者
func (l *Logic) resolveComment(ctx context.Context, commentID uint64, at time.Time) (NotifyInput, bool, error) {
	comment, err := l.comments.GetByID(ctx, commentID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotifyInput{}, false, nil
	}
	if err != nil {
		return NotifyInput{}, false, err
	}
	if comment.Commenter == 0 {
		return NotifyInput{}, false, nil
	}

	base := NotifyInput{
		TargetID:   comment.ID,
		ActorID:    comment.Commenter,
		VideoID:    comment.VideoID,
		Content:    summarize(comment.Content),
		OccurredAt: at,
	}
	if comment.ReplyToUserID > 0 {
		if comment.ReplyToUserID == comment.Commenter {
			return NotifyInput{}, false, nil
		}
		base.UserID = comment.ReplyToUserID
		base.Type = messagerepo.TypeReply
		return base, true, nil
	}

	video, err := l.videos.GetByID(ctx, comment.VideoID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotifyInput{}, false, nil
	}
	if err != nil {
		return NotifyInput{}, false, err
	}
	if video.AuthorID == 0 || video.AuthorID == comment.Commenter {
		return NotifyInput{}, false, nil
	}
	base.UserID = video.AuthorID
	base.Type = messagerepo.TypeComment
	return base, true, nil
}

// HandleFollowSwitched 关注事件产生通知。取关不通知，也不撤回已有通知
func (l *Logic) HandleFollowSwitched(ctx context.Context, payload []byte) error {
	var switched followevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if !switched.Followed {
		return nil
	}
	if switched.Follower == 0 || switched.Following == 0 {
		return consumer.Permanent(errors.New("关注事件里缺少用户 ID"))
	}
	input, ok := resolveFollow(switched.Follower, switched.Following, time.Now())
	if !ok {
		return nil
	}
	_, err := l.Notify(ctx, input)
	return err
}

// resolveFollow 关注产生的通知，接收者是被关注者，目标是关注者
func resolveFollow(follower uint64, following uint64, at time.Time) (NotifyInput, bool) {
	if follower == 0 || following == 0 || follower == following {
		return NotifyInput{}, false
	}
	return NotifyInput{
		UserID:     following,
		Type:       messagerepo.TypeFollow,
		TargetID:   follower,
		ActorID:    follower,
		OccurredAt: at,
	}, true
}

// NotifyInput 一条待产生的通知
type NotifyInput struct {
	UserID     uint64
	Type       string
	TargetID   uint64
	ActorID    uint64
	VideoID    uint64
	Content    string
	OccurredAt time.Time
}

// Notify 先写明细（唯一键做幂等门），是新插入的才累加会话，最后把明细挂到会话上。
// 返回是否新插入，维护命令用它统计补了多少条
func (l *Logic) Notify(ctx context.Context, input NotifyInput) (bool, error) {
	at := input.OccurredAt
	if at.IsZero() {
		at = time.Now()
	}
	detail := messagerepo.Message{
		UserID:   input.UserID,
		Type:     input.Type,
		TargetID: input.TargetID,
		ActorID:  input.ActorID,
		VideoID:  input.VideoID,
		Content:  input.Content,
	}
	messageID, inserted, err := l.repo.InsertDetail(ctx, &detail)
	if err != nil {
		slog.Error("写入通知明细失败", "user_id", input.UserID, "type", input.Type, "error", err)
		return false, err
	}
	if !inserted {
		// 这笔互动已经通知过，取消再点赞、消息重投、位点重放都走到这里
		return false, nil
	}

	thread := messagerepo.MessageThread{
		UserID:       input.UserID,
		Type:         input.Type,
		TargetID:     input.TargetID,
		WindowBucket: uint32(at.Unix() / windowSeconds),
		VideoID:      input.VideoID,
		ActorIDs:     actorIDString(input.ActorID),
		Content:      input.Content,
		LastAt:       at,
	}
	if err = l.repo.UpsertThread(ctx, &thread); err != nil {
		slog.Error("累加通知会话失败", "user_id", input.UserID, "type", input.Type, "error", err)
		return false, err
	}
	if err = l.repo.BindThread(ctx, messageID, &thread); err != nil {
		slog.Error("通知明细挂载会话失败", "message_id", messageID, "error", err)
		return false, err
	}
	return true, nil
}

// BackfillStats 补齐通知的统计
type BackfillStats struct {
	LikeVideo      int
	LikeComment    int
	Comment        int
	Reply          int
	Follow         int
	SkippedActor   int
	SkippedMissing int
}

// Backfill 按现有的点赞、评论、关注数据补齐站内通知。
// 通知的发生时间取关系行或评论行的创建时间，所以会话的窗口与列表顺序与真实互动时间一致
func (l *Logic) Backfill(ctx context.Context, batch int) (BackfillStats, error) {
	if batch <= 0 {
		batch = 500
	}
	stats := BackfillStats{}
	existingUsers, err := l.repo.ExistingUserIDs(ctx)
	if err != nil {
		return stats, err
	}

	// 点赞关系
	afterID := uint64(0)
	for {
		rows, err := l.repo.PageLikes(ctx, afterID, batch)
		if err != nil {
			return stats, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			afterID = row.ID
			var input NotifyInput
			var ok bool
			if row.TargetType == "video" {
				input, ok, err = l.resolveVideoLike(ctx, row.TargetID, row.UserID, row.CreatedAt)
			} else {
				input, ok, err = l.resolveCommentLike(ctx, row.TargetID, row.UserID, row.CreatedAt)
			}
			if err != nil {
				return stats, err
			}
			if !ok {
				stats.SkippedMissing++
				continue
			}
			if !existingUsers[input.UserID] {
				stats.SkippedActor++
				continue
			}
			inserted, err := l.Notify(ctx, input)
			if err != nil {
				return stats, err
			}
			if !inserted {
				continue
			}
			if input.Type == messagerepo.TypeLikeVideo {
				stats.LikeVideo++
			} else {
				stats.LikeComment++
			}
		}
		if len(rows) < batch {
			break
		}
	}

	// 评论与回复
	afterID = 0
	for {
		rows, err := l.repo.PageComments(ctx, afterID, batch)
		if err != nil {
			return stats, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			afterID = row.ID
			input, ok, err := l.resolveComment(ctx, row.ID, row.CreatedAt)
			if err != nil {
				return stats, err
			}
			if !ok {
				stats.SkippedMissing++
				continue
			}
			if !existingUsers[input.UserID] {
				stats.SkippedActor++
				continue
			}
			inserted, err := l.Notify(ctx, input)
			if err != nil {
				return stats, err
			}
			if !inserted {
				continue
			}
			if input.Type == messagerepo.TypeReply {
				stats.Reply++
			} else {
				stats.Comment++
			}
		}
		if len(rows) < batch {
			break
		}
	}

	// 关注关系
	afterID = 0
	for {
		rows, err := l.repo.PageFollows(ctx, afterID, batch)
		if err != nil {
			return stats, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			afterID = row.ID
			input, ok := resolveFollow(row.Follower, row.Following, row.CreatedAt)
			if !ok {
				continue
			}
			if !existingUsers[input.UserID] {
				stats.SkippedActor++
				continue
			}
			inserted, err := l.Notify(ctx, input)
			if err != nil {
				return stats, err
			}
			if inserted {
				stats.Follow++
			}
		}
		if len(rows) < batch {
			break
		}
	}

	return stats, nil
}

// ListMine 取当前用户的通知会话一页，双字段游标分页
func (l *Logic) ListMine(ctx context.Context, userID uint64, lastAt int64, lastID uint64, limit uint64) (ListRes, error) {
	limit = NormalizeListLimit(limit)
	var cursor time.Time
	if lastID > 0 {
		cursor = time.UnixMilli(lastAt)
	}

	threads, err := l.repo.ListThreads(ctx, userID, cursor, lastID, int(limit))
	if err != nil {
		return ListRes{}, err
	}
	if len(threads) == 0 {
		return ListRes{MessageList: []MessageItem{}}, nil
	}

	items, err := l.assemble(ctx, threads)
	if err != nil {
		return ListRes{}, err
	}
	last := threads[len(threads)-1]
	return ListRes{
		MessageList:   items,
		LastCreatedAt: last.LastAt.UnixMilli(),
		LastId:        last.ID,
	}, nil
}

// assemble 批量补上触发者信息与目标封面，避免每条消息各查一次
func (l *Logic) assemble(ctx context.Context, threads []messagerepo.MessageThread) ([]MessageItem, error) {
	actorIDSet := map[uint64]bool{}
	videoIDSet := map[uint64]bool{}
	threadActors := make([][]uint64, len(threads))
	for i, thread := range threads {
		for _, actorID := range messagerepo.SplitActorIDs(thread.ActorIDs) {
			threadActors[i] = append(threadActors[i], actorID)
			actorIDSet[actorID] = true
		}
		if thread.VideoID > 0 {
			videoIDSet[thread.VideoID] = true
		}
	}

	actorIDs := make([]uint64, 0, len(actorIDSet))
	for id := range actorIDSet {
		actorIDs = append(actorIDs, id)
	}
	videoIDs := make([]uint64, 0, len(videoIDSet))
	for id := range videoIDSet {
		videoIDs = append(videoIDs, id)
	}

	users, err := l.users.FilterByIDs(ctx, actorIDs)
	if err != nil {
		return nil, err
	}
	userByID := make(map[uint64]userrepo.User, len(users))
	for _, item := range users {
		userByID[item.ID] = item
	}

	videos, err := l.videos.FilterByIDs(ctx, videoIDs)
	if err != nil {
		return nil, err
	}
	coverByID := make(map[uint64]string, len(videos))
	for _, item := range videos {
		coverByID[item.ID] = l.uploader.URL(item.CoverURL)
	}

	items := make([]MessageItem, 0, len(threads))
	for i, thread := range threads {
		actors := make([]ActorItem, 0, len(threadActors[i]))
		for _, actorID := range threadActors[i] {
			user, ok := userByID[actorID]
			if !ok {
				continue
			}
			actors = append(actors, ActorItem{
				ID:        user.ID,
				Nickname:  user.NickName,
				AvatarURL: l.uploader.URL(user.AvatarURL),
			})
		}
		if len(actors) == 0 {
			continue
		}
		items = append(items, MessageItem{
			ID:         thread.ID,
			Type:       thread.Type,
			Title:      titleOf(thread.Type),
			Content:    thread.Content,
			Actors:     actors,
			ActorCount: thread.ActorCount,
			VideoID:    thread.VideoID,
			CommentID:  messagerepo.CommentIDOf(thread.Type, thread.TargetID),
			CoverURL:   coverByID[thread.VideoID],
			IsRead:     thread.IsRead,
			CreatedAt:  thread.LastAt,
		})
	}
	return items, nil
}

// Unread 未读会话数
func (l *Logic) Unread(ctx context.Context, userID uint64) (int64, error) {
	return l.repo.CountUnread(ctx, userID)
}

// MarkRead 标记已读，messageIDs 为空表示全部
func (l *Logic) MarkRead(ctx context.Context, userID uint64, messageIDs []uint64) (int64, error) {
	return l.repo.MarkRead(ctx, userID, messageIDs)
}

// titleOf 通知类型对应的主文案
func titleOf(msgType string) string {
	switch msgType {
	case messagerepo.TypeLikeVideo:
		return "收到点赞"
	case messagerepo.TypeLikeComment:
		return "收到评论点赞"
	case messagerepo.TypeComment:
		return "收到评论"
	case messagerepo.TypeReply:
		return "收到回复"
	case messagerepo.TypeFollow:
		return "新增关注"
	default:
		return "收到消息"
	}
}

func actorIDString(actorID uint64) string {
	return strconv.FormatUint(actorID, 10)
}

func millisToTime(millis int64) time.Time {
	if millis <= 0 {
		return time.Now()
	}
	return time.UnixMilli(millis)
}

// summarize 评论正文压缩成一行摘要
func summarize(content string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(content, "\n", " "))
	runes := []rune(trimmed)
	if len(runes) <= contentMaxRunes {
		return trimmed
	}
	return string(runes[:contentMaxRunes]) + "..."
}
