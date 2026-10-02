package feed

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	commentevent "simple_tiktok/internal/modules/comment/event"
	favoriteevent "simple_tiktok/internal/modules/favorite/event"
	favoriterepo "simple_tiktok/internal/modules/favorite/repo"
	feedrepo "simple_tiktok/internal/modules/feed/repo"
	followevent "simple_tiktok/internal/modules/follow/event"
	followrepo "simple_tiktok/internal/modules/follow/repo"
	likeevent "simple_tiktok/internal/modules/like/event"
	likerepo "simple_tiktok/internal/modules/like/repo"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videoevent "simple_tiktok/internal/modules/video/event"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/upload"
)

const (
	likeHotDelta     = 2
	favoriteHotDelta = 3
	commentHotDelta  = 1

	// 查询窗口的上限取热度数据的保留时长，声明的窗口不会超过实际留得住的数据。
	// 窗口按分钟滑动，所以榜单的可见刷新间隔是一分钟
	defaultHotInterval = 60
	maxHotInterval     = feedrepo.HotRetentionMinutes

	defaultHotLimit = 3
	maxHotLimit     = 50

	// 计分类型，标记键按它区分点赞、收藏与评论
	hotTypeLike     = "like"
	hotTypeFavorite = "favorite"
	hotTypeComment  = "comment"

	// bigAuthorFollowers 大号阈值。粉丝数达到它的作者发布时只写自己的发件箱，
	// 由粉丝读取时现取；低于它就逐个推进粉丝的收件箱。
	// 取值依据是一次发布的写扩散预算：最坏情况下一条发布最多写这么多次收件箱
	bigAuthorFollowers = 5000

	// 扇出时每批取多少个粉丝写一次管道
	followFanoutBatch = 500

	// 关注时回填的历史内容条数
	followBackfillLimit = 20

	// 关注流读取时多取的候选条数，补上已删除内容被过滤后造成的空缺
	followCandidateExtra = 20
)

// Logic Feed 模块的业务逻辑，负责索引与热度，视频与用户数据都通过别人的 repo 取
type Logic struct {
	feed      *feedrepo.Repo
	videos    *videorepo.Repo
	users     *userrepo.Repo
	likes     *likerepo.Repo
	favorites *favoriterepo.Repo
	follows   *followrepo.Repo
	uploader  *upload.Uploader
}

// NormalizeHotQuery 归一化热榜参数：记录不存在时取默认值，超出上限时取上限。
// 归一化的结果要回给客户端，避免响应里声明的窗口与实际使用的窗口不一致
func NormalizeHotQuery(limit uint64, interval int) (uint64, int) {
	if limit == 0 {
		limit = defaultHotLimit
	}
	if limit > maxHotLimit {
		limit = maxHotLimit
	}
	if interval <= 0 {
		interval = defaultHotInterval
	}
	if interval > maxHotInterval {
		interval = maxHotInterval
	}
	return limit, interval
}

// GetFeedVideos 按发布时间倒序取一页，双字段游标分页，第一页 lastId 传 0
func (l *Logic) GetFeedVideos(ctx context.Context, limit uint64, lastCreatedAt int64, lastId uint64, userID uint64) ([]VideoItem, int64, uint64, error) {
	var cursor time.Time
	if lastId > 0 {
		cursor = time.UnixMilli(lastCreatedAt)
	}
	items, err := l.videos.ListFeedPage(ctx, limit, cursor, lastId)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(items) == 0 {
		return []VideoItem{}, 0, 0, nil
	}
	list, err := l.assemble(ctx, items, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	last := list[len(list)-1]
	return list, last.CreatedAt.UnixMilli(), last.Id, nil
}

// GetHotVideos 按热度倒序取一页。分数只由互动产生，窗口内的分数按分钟累加，
// 衰减由桶过期完成，所以不需要任何定时任务
func (l *Logic) GetHotVideos(ctx context.Context, limit uint64, offset uint64, interval int, userID uint64) ([]VideoItem, uint64, bool, error) {
	limit, interval = NormalizeHotQuery(limit, interval)

	items, consumed, hasMore, err := l.feed.HotPage(ctx, limit, offset, interval)
	if err != nil {
		return nil, offset, false, err
	}
	if len(items) == 0 {
		return []VideoItem{}, offset, false, nil
	}

	ids := make([]uint64, len(items))
	scores := make(map[uint64]float64, len(items))
	for i, item := range items {
		ids[i] = item.ID
		scores[item.ID] = item.Score
	}

	rows, err := l.videos.FilterByIDs(ctx, ids)
	if err != nil {
		return nil, offset, false, err
	}
	// 行已经不存在的成员在这一步被丢掉，页面会少几条，游标按实际取到的条数前进
	ordered := orderVideos(rows, scores)
	if len(ordered) == 0 {
		return []VideoItem{}, offset + consumed, hasMore, nil
	}

	list, err := l.assemble(ctx, ordered, userID)
	if err != nil {
		return nil, offset, false, err
	}
	for i := range list {
		list[i].Score = scores[list[i].Id]
	}
	return list, offset + consumed, hasMore, nil
}

// GetFollowFeedVideos 取关注的人发布的视频，双字段游标分页，第一页 lastId 传 0。
// 优先读推拉结合建起来的索引：小号的内容在我的收件箱里，
// 大号的内容去他的发件箱现取；索引不可用时退回按作者查表
func (l *Logic) GetFollowFeedVideos(ctx context.Context, limit uint64, lastCreatedAt int64, lastId uint64, userID uint64) ([]VideoItem, int64, uint64, error) {
	followingIDs, err := l.follows.FollowingIDs(ctx, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(followingIDs) == 0 {
		return []VideoItem{}, 0, 0, nil
	}

	var cursor time.Time
	if lastId > 0 {
		cursor = time.UnixMilli(lastCreatedAt)
	}

	items, indexed, err := l.feed.FollowPage(ctx, userID, followingIDs, cursor, lastId, int(limit)+followCandidateExtra)
	if err != nil {
		return nil, 0, 0, err
	}
	if !indexed {
		return l.listFollowFeedFromTable(ctx, followingIDs, limit, cursor, lastId, userID)
	}

	ids := make([]uint64, len(items))
	scores := make(map[uint64]float64, len(items))
	for i, item := range items {
		ids[i] = item.ID
		scores[item.ID] = item.Score
	}
	// 行已经被删除的成员在这一步丢掉，列表按索引里的顺序恢复
	rows, err := l.videos.FilterByIDs(ctx, ids)
	if err != nil {
		return nil, 0, 0, err
	}
	ordered := orderVideos(rows, scores)
	if len(ordered) == 0 {
		// 这一页索引里的内容全部已经不可见，退回查表把后面还没进索引的内容补上
		return l.listFollowFeedFromTable(ctx, followingIDs, limit, cursor, lastId, userID)
	}
	if uint64(len(ordered)) > limit {
		ordered = ordered[:limit]
	}
	list, err := l.assemble(ctx, ordered, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(list) == 0 {
		return []VideoItem{}, 0, 0, nil
	}
	last := list[len(list)-1]
	return list, last.CreatedAt.UnixMilli(), last.Id, nil
}

// listFollowFeedFromTable 索引里没有任何数据时按作者集合查表
func (l *Logic) listFollowFeedFromTable(ctx context.Context, followingIDs []uint64, limit uint64, cursor time.Time, lastId uint64, userID uint64) ([]VideoItem, int64, uint64, error) {
	items, err := l.videos.ListByAuthorsBefore(ctx, followingIDs, limit, cursor, lastId)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(items) == 0 {
		return []VideoItem{}, 0, 0, nil
	}
	list, err := l.assemble(ctx, items, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	last := list[len(list)-1]
	return list, last.CreatedAt.UnixMilli(), last.Id, nil
}

// HandleVideoCreated 视频发布后写关注流索引：大号只写自己的发件箱，
// 小号按粉丝分批推进每个人的收件箱
func (l *Logic) HandleVideoCreated(ctx context.Context, payload []byte) error {
	var created videoevent.CreatedEvent
	if err := json.Unmarshal(payload, &created); err != nil {
		return consumer.Permanent(err)
	}
	if created.VideoID == 0 || created.AuthorID == 0 {
		return consumer.Permanent(errors.New("创建视频事件里缺少视频或作者 ID"))
	}
	publishedAt := created.CreatedAt
	if publishedAt.IsZero() {
		publishedAt = time.Now()
	}
	item := feedrepo.InboxItem{
		VideoID:     created.VideoID,
		AuthorID:    created.AuthorID,
		PublishedAt: publishedAt,
	}

	// 粉丝数取反向集合的基数，集合不存在按大号处理：只写发件箱，避免给大批粉丝写收件箱
	followers, err := l.follows.CountFollowers(ctx, created.AuthorID)
	if err != nil && !errors.Is(err, redis.Nil) {
		slog.Error("统计作者粉丝数失败", "author_id", created.AuthorID, "error", err)
		return err
	}
	if followers == 0 || followers >= bigAuthorFollowers {
		if err := l.feed.PushOutboxItem(ctx, item); err != nil {
			slog.Error("写发件箱失败", "author_id", created.AuthorID, "video_id", created.VideoID, "error", err)
			return err
		}
		return nil
	}
	return l.fanoutToInboxes(ctx, created.AuthorID, item)
}

// fanoutToInboxes 用 SSCAN 分批取粉丝，逐批写收件箱。
// 同一个作者的事件进同一个分区、由一个消费循环串行处理，扇出顺序与发布顺序一致
func (l *Logic) fanoutToInboxes(ctx context.Context, authorID uint64, item feedrepo.InboxItem) error {
	cursor := uint64(0)
	for {
		followerIDs, next, err := l.follows.FollowerIDsPage(ctx, authorID, cursor, followFanoutBatch)
		if err != nil {
			slog.Error("取粉丝列表失败", "author_id", authorID, "error", err)
			return err
		}
		if err = l.feed.PushInboxItems(ctx, followerIDs, item); err != nil {
			slog.Error("写收件箱失败", "author_id", authorID, "video_id", item.VideoID, "error", err)
			return err
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}

// HandleFollowSwitched 关注成功后回填被关注者的最近内容，
// 让关注流立刻有东西可看；取关不需要清理收件箱，读取时会按关注状态过滤
func (l *Logic) HandleFollowSwitched(ctx context.Context, payload []byte) error {
	var switched followevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Follower == 0 || switched.Following == 0 {
		return consumer.Permanent(errors.New("关注事件里缺少用户 ID"))
	}
	if !switched.Followed {
		return nil
	}

	rows, err := l.videos.ListByAuthorsBefore(ctx, []uint64{switched.Following}, followBackfillLimit, time.Time{}, 0)
	if err != nil {
		slog.Error("回填关注流失败", "follower", switched.Follower, "following", switched.Following, "error", err)
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	items := make([]feedrepo.InboxItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, feedrepo.InboxItem{
			VideoID:     row.ID,
			AuthorID:    row.AuthorID,
			PublishedAt: row.CreateTime,
		})
	}
	if err = l.feed.PushInboxItemsForFollower(ctx, switched.Follower, items); err != nil {
		slog.Error("回填收件箱失败", "follower", switched.Follower, "error", err)
		return err
	}
	return nil
}

// score 给视频计一次分。事件时间已经超出保留时长的直接丢弃，
// 窗口内同一个用户的同类互动由标记键保证只计一次
func (l *Logic) score(ctx context.Context, scoreType string, videoID uint64, userID uint64, weight float64, occurredAt int64) error {
	now := time.Now()
	at := now
	if occurredAt > 0 {
		at = time.UnixMilli(occurredAt)
	}
	if at.After(now) {
		// 时钟轻微偏差时按当前时刻处理，不写未来的桶
		at = now
	}
	if now.Sub(at) >= feedrepo.HotRetention {
		// 已经超出保留窗口，写进去也不会被任何查询窗口读到
		return nil
	}

	scored, err := l.feed.ScoreOnce(ctx, scoreType, videoID, userID, weight, at, now)
	if err != nil {
		slog.Error("热度计分失败", "type", scoreType, "video_id", videoID, "error", err)
		return err
	}
	if !scored {
		slog.Debug("窗口内已经计过分，跳过", "type", scoreType, "video_id", videoID, "user_id", userID)
	}
	return nil
}

// HandleVideoDeleted 把视频从保留窗口覆盖的热度桶里清掉
func (l *Logic) HandleVideoDeleted(ctx context.Context, payload []byte) error {
	var deleted videoevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.VideoID == 0 {
		return consumer.Permanent(errors.New("删除视频事件里没有 videoId"))
	}
	return l.feed.RemoveFromHotBuckets(ctx, deleted.VideoID, feedrepo.HotRetentionMinutes)
}

// HandleLikeSwitched 只处理视频点赞，评论点赞不影响视频热度。
// 取消点赞不回减分数，窗口内已经计入的那一份保留到桶自然过期
func (l *Logic) HandleLikeSwitched(ctx context.Context, payload []byte) error {
	var switched likeevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Target != likeevent.TargetVideo {
		return nil
	}
	if !switched.Liked {
		return nil
	}
	if switched.TargetID == 0 || switched.Operator == 0 {
		return consumer.Permanent(errors.New("点赞事件里缺少用户或目标 ID"))
	}
	return l.score(ctx, hotTypeLike, switched.TargetID, switched.Operator, likeHotDelta, switched.OccurredAt)
}

// HandleFavoriteSwitched 收藏加热度，取消收藏不回减
func (l *Logic) HandleFavoriteSwitched(ctx context.Context, payload []byte) error {
	var switched favoriteevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.VideoID == 0 || switched.UserID == 0 {
		return consumer.Permanent(errors.New("收藏事件里缺少用户或视频 ID"))
	}
	if !switched.Favorited {
		return nil
	}
	return l.score(ctx, hotTypeFavorite, switched.VideoID, switched.UserID, favoriteHotDelta, switched.OccurredAt)
}

// HandleCommentCreated 评论创建加热度。评论删除不回减
func (l *Logic) HandleCommentCreated(ctx context.Context, payload []byte) error {
	var created commentevent.CreatedEvent
	if err := json.Unmarshal(payload, &created); err != nil {
		return consumer.Permanent(err)
	}
	if created.VideoID == 0 || created.Commenter == 0 {
		return consumer.Permanent(errors.New("评论事件里缺少用户或视频 ID"))
	}
	return l.score(ctx, hotTypeComment, created.VideoID, created.Commenter, commentHotDelta, created.OccurredAt)
}

func (l *Logic) assemble(ctx context.Context, items []videorepo.Video, userID uint64) ([]VideoItem, error) {
	list := make([]VideoItem, len(items))
	authorIDs := make([]uint64, 0, len(items))
	videoIDs := make([]uint64, len(items))
	seen := make(map[uint64]bool, len(items))
	for i, item := range items {
		list[i] = VideoItem{
			Id:            item.ID,
			AuthorID:      item.AuthorID,
			AuthorName:    item.AuthorName,
			AuthorAvatar:  l.uploader.URL(item.AuthorAvatar),
			Title:         item.Title,
			Description:   item.Description,
			CoverURL:      l.uploader.URL(item.CoverURL),
			PlayURL:       l.uploader.URL(item.PlayURL),
			LikeCount:     item.LikeCount,
			CommentCount:  item.CommentCount,
			FavoriteCount: item.FavoriteCount,
			CreatedAt:     item.CreateTime,
		}
		videoIDs[i] = item.ID
		if item.AuthorID != 0 && !seen[item.AuthorID] {
			seen[item.AuthorID] = true
			authorIDs = append(authorIDs, item.AuthorID)
		}
	}

	targets := make([]uint64, 0, len(authorIDs))
	for _, authorID := range authorIDs {
		if authorID != userID {
			targets = append(targets, authorID)
		}
	}

	// 三批状态查询合并到一条管道，一次往返取回
	pipe := l.feed.Pipeline()
	likedCmds := l.likes.AppendLiked(pipe, ctx, likeevent.TargetVideo, userID, videoIDs)
	favoritedCmds := l.favorites.AppendFavorited(pipe, ctx, userID, videoIDs)
	followCmds := l.follows.AppendFollowing(pipe, ctx, userID, targets)
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	liked, err := boolResults(likedCmds)
	if err != nil {
		return nil, err
	}
	favorited, err := boolResults(favoritedCmds)
	if err != nil {
		return nil, err
	}
	followed, err := boolResults(followCmds)
	if err != nil {
		return nil, err
	}

	followedByAuthor := make(map[uint64]bool, len(targets))
	for i, authorID := range targets {
		followedByAuthor[authorID] = followed[i]
	}
	for i := range list {
		list[i].IsLiked = liked[i]
		list[i].IsFavorited = favorited[i]
		if list[i].AuthorID == 0 || list[i].AuthorID == userID {
			list[i].IsFollow = false
			continue
		}
		list[i].IsFollow = followedByAuthor[list[i].AuthorID]
	}
	return list, nil
}

// boolResults 取出一批布尔命令的结果，键不存在按 false 处理
func boolResults(commands []*redis.BoolCmd) ([]bool, error) {
	out := make([]bool, len(commands))
	for i, cmd := range commands {
		value, err := cmd.Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		out[i] = value
	}
	return out, nil
}

// orderVideos 按分数倒序排定，同分时用发布时间与 ID 定序，
// 不让有序集合在分数相同时按成员字符串的字典序决定先后
func orderVideos(items []videorepo.Video, scores map[uint64]float64) []videorepo.Video {
	ordered := make([]videorepo.Video, len(items))
	copy(ordered, items)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if scores[left.ID] != scores[right.ID] {
			return scores[left.ID] > scores[right.ID]
		}
		if !left.CreateTime.Equal(right.CreateTime) {
			return left.CreateTime.After(right.CreateTime)
		}
		return left.ID > right.ID
	})
	return ordered
}
