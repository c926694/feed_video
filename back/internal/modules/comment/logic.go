package comment

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
	likeevent "simple_tiktok/internal/modules/like/event"
	likerepo "simple_tiktok/internal/modules/like/repo"
	userevent "simple_tiktok/internal/modules/user/event"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videoevent "simple_tiktok/internal/modules/video/event"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/platform/upload"
)

const (
	defaultCommentLimit = 20
	maxCommentLimit     = 100
)

// Logic 评论模块的业务逻辑
type Logic struct {
	comments *commentrepo.Repo
	videos   *videorepo.Repo
	users    *userrepo.Repo
	likes    *likerepo.Repo
	producer *producer.Producer
	uploader *upload.Uploader
}

// Create 发表评论或回复。父评论与回复对象都必须属于同一个视频，
// 被回复者的昵称按创建时的快照写入，不随改名刷新
func (l *Logic) Create(ctx context.Context, userID uint64, createReq CreateReq) (*InfoRes, error) {
	content := strings.TrimSpace(createReq.Content)
	if content == "" {
		return nil, httpx.New(httpx.CodeBadRequest, "评论内容不能为空")
	}
	video, err := l.videos.GetByID(ctx, createReq.VideoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.New(httpx.CodeNotFound, "视频不存在")
		}
		return nil, err
	}
	if video.Status != videorepo.StatusPublished {
		return nil, httpx.New(httpx.CodeNotFound, "视频不存在")
	}

	item := commentrepo.Comment{
		Content:   content,
		VideoID:   createReq.VideoID,
		Commenter: userID,
	}
	if createReq.ParentID > 0 {
		parent, getErr := l.comments.GetByID(ctx, createReq.ParentID)
		if getErr != nil {
			if errors.Is(getErr, gorm.ErrRecordNotFound) {
				return nil, httpx.New(httpx.CodeNotFound, "父评论不存在")
			}
			return nil, getErr
		}
		if parent.VideoID != createReq.VideoID {
			return nil, httpx.New(httpx.CodeBadRequest, "父评论不属于该视频")
		}
		target := parent
		if createReq.ReplyToID > 0 && createReq.ReplyToID != parent.ID {
			target, getErr = l.comments.GetByID(ctx, createReq.ReplyToID)
			if getErr != nil {
				if errors.Is(getErr, gorm.ErrRecordNotFound) {
					return nil, httpx.New(httpx.CodeNotFound, "被回复的评论不存在")
				}
				return nil, getErr
			}
			if target.VideoID != createReq.VideoID {
				return nil, httpx.New(httpx.CodeBadRequest, "被回复的评论不属于该视频")
			}
		}
		item.ParentID = parent.ID
		item.ReplyToID = target.ID
		item.ReplyToUserID = target.Commenter
		item.ReplyToUserName = target.CommenterName
	}

	commenterName := ""
	commenterAvatar := ""
	if user, userErr := l.users.GetByID(ctx, userID); userErr == nil {
		commenterName = user.NickName
		commenterAvatar = user.AvatarURL
	}
	item.CommenterName = commenterName
	item.CommenterAvatar = commenterAvatar

	var createErr error
	if item.ParentID > 0 {
		createErr = l.comments.CreateReply(ctx, &item, item.ParentID, item.VideoID)
	} else {
		createErr = l.comments.CreateTop(ctx, &item, item.VideoID)
	}
	if createErr != nil {
		if errors.Is(createErr, gorm.ErrRecordNotFound) {
			return nil, httpx.New(httpx.CodeNotFound, "父评论不存在")
		}
		return nil, createErr
	}

	// 热度走 Kafka，投递失败只记日志；评论数已随事务即时更新。
	// 评论数对视频展示不重要，缓存里的旧值由逻辑过期兜住，不主动失效
	if err := l.producer.Publish(ctx, topic.CommentCreated, strconv.FormatUint(item.ID, 10), commentevent.CreatedEvent{
		CommentID: item.ID,
		VideoID:   item.VideoID,
		Commenter: item.Commenter,
	}); err != nil {
		slog.Error("发布评论创建事件失败", "comment_id", item.ID, "error", err)
	}

	info := l.toInfoRes(item)
	return &info, nil
}

// Delete 删除评论。顶级评论连同整楼子评论一起删除；
// 删除不存在的评论视为成功，保持幂等。评论行、计数、缓存、点赞关系全部同步处理，
// 只有热度这类纯增量副作用走事件，删除不发事件
func (l *Logic) Delete(ctx context.Context, userID uint64, commentID uint64) error {
	item, err := l.comments.GetByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 已经删除过，幂等成功
			return nil
		}
		return err
	}
	if item.Commenter != userID {
		return httpx.New(httpx.CodeForbidden, "只能删除自己的评论")
	}

	if item.ParentID > 0 {
		// 删除单条子评论：同一事务里扣减父评论回复数与视频评论数
		if err := l.comments.DeleteReply(ctx, item.ID, item.ParentID, item.VideoID); err != nil {
			return err
		}
		l.clearCommentLikes(ctx, []uint64{item.ID})
		return nil
	}

	// 删除整楼：事务里删掉楼内全部行并扣减视频评论数，返回被删的行批量清点赞
	removed, err := l.comments.DeleteTopWithReplies(ctx, item.ID, item.VideoID)
	if err != nil {
		return err
	}
	commentIDs := make([]uint64, 0, len(removed))
	for _, row := range removed {
		commentIDs = append(commentIDs, row.ID)
	}
	l.clearCommentLikes(ctx, commentIDs)
	return nil
}

// clearCommentLikes 批量清理点赞关系与点赞集合：一条 SQL 加一条 DEL，幂等操作失败只记日志
func (l *Logic) clearCommentLikes(ctx context.Context, commentIDs []uint64) {
	if len(commentIDs) == 0 {
		return
	}
	if err := l.likes.DeleteByTargets(ctx, likeevent.TargetComment, commentIDs); err != nil {
		slog.Error("批量清理评论点赞关系失败", "comment_count", len(commentIDs), "error", err)
	}
	if err := l.likes.DeleteTargetSets(ctx, likeevent.TargetComment, commentIDs); err != nil {
		slog.Error("批量清理评论点赞集合失败", "comment_count", len(commentIDs), "error", err)
	}
}

// ListByVideo 顶级评论分页，只返回顶层与 reply_count，子评论走回复接口按需加载
func (l *Logic) ListByVideo(ctx context.Context, videoID uint64, lastCreatedAt int64, lastID uint64, limit uint64, userID uint64) (*ListRes, error) {
	limit = normalizeLimit(limit)
	var cursor time.Time
	if lastID > 0 {
		cursor = time.UnixMilli(lastCreatedAt)
	}

	items, err := l.comments.ListTopPage(ctx, videoID, cursor, lastID, int(limit)+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(items) > int(limit)
	if hasMore {
		items = items[:limit]
	}

	list := make([]InfoRes, 0, len(items))
	for i := range items {
		list = append(list, l.toInfoRes(items[i]))
	}

	if err = l.fillLiked(ctx, list, userID); err != nil {
		return nil, err
	}

	result := &ListRes{List: list, HasMore: hasMore}
	if len(items) > 0 {
		last := items[len(items)-1]
		result.LastCreatedAt = last.CreateTime.UnixMilli()
		result.LastID = last.ID
	}
	return result, nil
}

// ListReplies 某个顶级评论下子评论的分页。传子评论 ID 时按它所属的楼处理
func (l *Logic) ListReplies(ctx context.Context, commentID uint64, lastCreatedAt int64, lastID uint64, limit uint64, userID uint64) (*ListRes, error) {
	anchor, err := l.comments.GetByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.New(httpx.CodeNotFound, "评论不存在")
		}
		return nil, err
	}
	parentID := anchor.ID
	if anchor.ParentID > 0 {
		parentID = anchor.ParentID
	}

	limit = normalizeLimit(limit)
	var cursor time.Time
	if lastID > 0 {
		cursor = time.UnixMilli(lastCreatedAt)
	}

	items, err := l.comments.ListRepliesPage(ctx, anchor.VideoID, parentID, cursor, lastID, int(limit)+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(items) > int(limit)
	if hasMore {
		items = items[:limit]
	}

	list := make([]InfoRes, 0, len(items))
	for i := range items {
		list = append(list, l.toInfoRes(items[i]))
	}
	if err = l.fillLiked(ctx, list, userID); err != nil {
		return nil, err
	}

	result := &ListRes{List: list, HasMore: hasMore}
	if len(items) > 0 {
		last := items[len(items)-1]
		result.LastCreatedAt = last.CreateTime.UnixMilli()
		result.LastID = last.ID
	}
	return result, nil
}

// toInfoRes 把评论行转成响应，作者信息来自行上的冗余字段，不读 user 表
func (l *Logic) toInfoRes(item commentrepo.Comment) InfoRes {
	info := InfoRes{
		Id:              item.ID,
		VideoId:         item.VideoID,
		ParentId:        item.ParentID,
		ReplyToId:       item.ReplyToID,
		ReplyToUserID:   item.ReplyToUserID,
		ReplyToUserName: item.ReplyToUserName,
		Commenter:       item.Commenter,
		Content:         item.Content,
		LikeCount:       item.LikeCount,
		ReplyCount:      item.ReplyCount,
		CreatedAt:       item.CreateTime,
		commenterName:   item.CommenterName,
		commenterAvatar: item.CommenterAvatar,
	}
	info.Author = AuthorRes{
		UserID:    item.Commenter,
		Nickname:  item.CommenterName,
		AvatarURL: l.uploader.URL(item.CommenterAvatar),
	}
	return info
}

// HandleLikeSwitched 订阅点赞事件，只处理评论点赞。
// 对账数据源是 Redis 集合：集合在请求路径里同步写入，
// 事件到达消费方时集合一定是新的，没有跨消费组的竞态
func (l *Logic) HandleLikeSwitched(ctx context.Context, payload []byte) error {
	var switched likeevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Target != likeevent.TargetComment {
		return nil
	}
	if switched.TargetID == 0 {
		return consumer.Permanent(errors.New("点赞事件里没有 targetId"))
	}

	count, err := l.likes.CountLikes(ctx, likeevent.TargetComment, switched.TargetID)
	if err != nil {
		slog.Error("统计评论点赞数失败", "comment_id", switched.TargetID, "error", err)
		return err
	}
	if err = l.comments.SyncLikeCount(ctx, switched.TargetID, count); err != nil {
		slog.Error("对账评论点赞数失败", "comment_id", switched.TargetID, "error", err)
		return err
	}
	return nil
}

// HandleVideoDeleted 视频被删除后清理它下面的评论
func (l *Logic) HandleVideoDeleted(ctx context.Context, payload []byte) error {
	var deleted videoevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.VideoID == 0 {
		return consumer.Permanent(errors.New("删除视频事件里没有 videoId"))
	}
	if err := l.comments.DeleteByVideo(ctx, deleted.VideoID); err != nil {
		slog.Error("删除视频评论失败", "video_id", deleted.VideoID, "error", err)
		return err
	}
	return nil
}

// HandleUserUpdated 订阅用户资料变更，刷新自己表里冗余的评论者展示字段
func (l *Logic) HandleUserUpdated(ctx context.Context, payload []byte) error {
	var updated userevent.UpdatedEvent
	if err := json.Unmarshal(payload, &updated); err != nil {
		return consumer.Permanent(err)
	}
	if updated.UserID == 0 {
		return consumer.Permanent(errors.New("用户资料事件里没有 userId"))
	}
	if err := l.comments.UpdateCommenterInfo(ctx, updated.UserID, updated.Nickname, updated.AvatarURL); err != nil {
		slog.Error("刷新评论者信息失败", "commenter", updated.UserID, "error", err)
		return err
	}
	return nil
}

// fillLiked 批量补当前用户对评论的点赞状态
func (l *Logic) fillLiked(ctx context.Context, list []InfoRes, userID uint64) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]uint64, len(list))
	for i := range list {
		ids[i] = list[i].Id
	}
	liked, err := l.likes.FilterLiked(ctx, likeevent.TargetComment, userID, ids)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].IsLiked = liked[list[i].Id]
	}
	return nil
}

func normalizeLimit(limit uint64) uint64 {
	if limit <= 0 {
		return defaultCommentLimit
	}
	if limit > maxCommentLimit {
		return maxCommentLimit
	}
	return limit
}
