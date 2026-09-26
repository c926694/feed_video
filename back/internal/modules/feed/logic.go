package feed

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	commentevent "simple_tiktok/internal/modules/comment/event"
	feedrepo "simple_tiktok/internal/modules/feed/repo"
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
	likeHotDelta       = 2
	commentHotDelta    = 1
	defaultHotInterval = 60
	maxHotInterval     = 1440
)

// Logic Feed 模块的业务逻辑，负责索引与热度，视频与用户数据都通过别人的 repo 取
type Logic struct {
	feed     *feedrepo.Repo
	videos   *videorepo.Repo
	users    *userrepo.Repo
	likes    *likerepo.Repo
	follows  *followrepo.Repo
	uploader *upload.Uploader
}

// GetFeedVideos 按发布时间倒序取一页
func (l *Logic) GetFeedVideos(ctx context.Context, limit uint64, lastScore float64, userID uint64) ([]VideoItem, float64, error) {
	ids, err := l.feed.FeedIDs(ctx, limit, lastScore)
	if err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return []VideoItem{}, 0, nil
	}
	items, err := l.videos.FilterByIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	ordered := orderVideos(items, ids)
	if len(ordered) == 0 {
		return []VideoItem{}, 0, nil
	}
	list, err := l.assemble(ctx, ordered, userID)
	if err != nil {
		return nil, 0, err
	}
	return list, float64(list[len(list)-1].CreatedAt.UnixMicro()), nil
}

// GetHotVideos 按热度倒序取一页
func (l *Logic) GetHotVideos(ctx context.Context, limit uint64, offset uint64, interval int, userID uint64) ([]VideoItem, uint64, bool, error) {
	if limit == 0 {
		limit = 5
	}
	if interval <= 0 {
		interval = defaultHotInterval
	}
	if interval > maxHotInterval {
		interval = maxHotInterval
	}

	ids, consumed, hasMore, err := l.feed.HotIDs(ctx, limit, offset, interval)
	if err != nil {
		return nil, offset, false, err
	}
	if len(ids) == 0 {
		return []VideoItem{}, offset, false, nil
	}
	items, err := l.videos.FilterByIDs(ctx, ids)
	if err != nil {
		return nil, offset, false, err
	}
	ordered := orderVideos(items, ids)
	if len(ordered) == 0 {
		return []VideoItem{}, offset + consumed, hasMore, nil
	}
	list, err := l.assemble(ctx, ordered, userID)
	if err != nil {
		return nil, offset, false, err
	}
	return list, offset + consumed, hasMore, nil
}

// GetFollowFeedVideos 取关注的人发布的视频
func (l *Logic) GetFollowFeedVideos(ctx context.Context, limit uint64, lastScore float64, userID uint64) ([]VideoItem, float64, error) {
	followingIDs, err := l.follows.FollowingIDs(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	if len(followingIDs) == 0 {
		return []VideoItem{}, 0, nil
	}

	var before *time.Time
	if lastScore > 0 {
		t := time.UnixMicro(int64(lastScore))
		before = &t
	}
	items, err := l.videos.ListByAuthorsBefore(ctx, followingIDs, limit, before)
	if err != nil {
		return nil, 0, err
	}
	if len(items) == 0 {
		return []VideoItem{}, 0, nil
	}
	list, err := l.assemble(ctx, items, userID)
	if err != nil {
		return nil, 0, err
	}
	return list, float64(list[len(list)-1].CreatedAt.UnixMicro()), nil
}

// HandleVideoCreated 新视频进 Feed 索引与当前分钟的热度桶
func (l *Logic) HandleVideoCreated(ctx context.Context, payload []byte) error {
	var created videoevent.CreatedEvent
	if err := json.Unmarshal(payload, &created); err != nil {
		return consumer.Permanent(err)
	}
	if created.VideoID == 0 {
		return consumer.Permanent(errors.New("视频创建事件里没有 videoId"))
	}
	createdAt := created.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	if err := l.feed.AddToFeed(ctx, created.VideoID, createdAt); err != nil {
		slog.Error("加入 Feed 索引失败", "video_id", created.VideoID, "error", err)
		return err
	}
	if err := l.feed.EnsureHotMember(ctx, created.VideoID, time.Now()); err != nil {
		slog.Error("加入热度桶失败", "video_id", created.VideoID, "error", err)
		return err
	}
	return nil
}

// HandleVideoDeleted 把视频从 Feed 索引与热度桶里清掉
func (l *Logic) HandleVideoDeleted(ctx context.Context, payload []byte) error {
	var deleted videoevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.VideoID == 0 {
		return consumer.Permanent(errors.New("删除视频事件里没有 videoId"))
	}
	if err := l.feed.RemoveFromFeed(ctx, deleted.VideoID); err != nil {
		slog.Error("移出 Feed 索引失败", "video_id", deleted.VideoID, "error", err)
		return err
	}
	return l.feed.RemoveFromHotBuckets(ctx, deleted.VideoID, maxHotInterval)
}

// HandleLikeSwitched 只处理视频点赞，评论点赞不影响视频热度
func (l *Logic) HandleLikeSwitched(ctx context.Context, payload []byte) error {
	var switched likeevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Target != likeevent.TargetVideo {
		return nil
	}
	if switched.TargetID == 0 {
		return consumer.Permanent(errors.New("点赞事件里没有 targetId"))
	}
	delta := float64(likeHotDelta)
	if !switched.Liked {
		delta = -delta
	}
	return l.feed.IncreaseHotScore(ctx, switched.TargetID, delta, time.Now())
}

// HandleCommentCreated 评论创建加热度
func (l *Logic) HandleCommentCreated(ctx context.Context, payload []byte) error {
	var created commentevent.CreatedEvent
	if err := json.Unmarshal(payload, &created); err != nil {
		return consumer.Permanent(err)
	}
	if created.VideoID == 0 {
		return consumer.Permanent(errors.New("评论事件里没有 videoId"))
	}
	return l.feed.IncreaseHotScore(ctx, created.VideoID, commentHotDelta, time.Now())
}

// HandleCommentDeleted 评论删除减热度
func (l *Logic) HandleCommentDeleted(ctx context.Context, payload []byte) error {
	var deleted commentevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.VideoID == 0 {
		return consumer.Permanent(errors.New("评论事件里没有 videoId"))
	}
	return l.feed.IncreaseHotScore(ctx, deleted.VideoID, -commentHotDelta, time.Now())
}

func (l *Logic) assemble(ctx context.Context, items []videorepo.Video, userID uint64) ([]VideoItem, error) {
	list := make([]VideoItem, len(items))
	authorIDs := make([]uint64, 0, len(items))
	videoIDs := make([]uint64, len(items))
	seen := make(map[uint64]bool, len(items))
	for i, item := range items {
		list[i] = VideoItem{
			Id:           item.ID,
			AuthorID:     item.AuthorID,
			AuthorName:   item.AuthorName,
			AuthorAvatar: l.uploader.URL(item.AuthorAvatar),
			Title:        item.Title,
			Description:  item.Description,
			CoverURL:     l.uploader.URL(item.CoverURL),
			PlayURL:      l.uploader.URL(item.PlayURL),
			LikeCount:    item.LikeCount,
			CommentCount: item.CommentCount,
			CreatedAt:    item.CreatedAt,
		}
		videoIDs[i] = item.ID
		if item.AuthorID != 0 && !seen[item.AuthorID] {
			seen[item.AuthorID] = true
			authorIDs = append(authorIDs, item.AuthorID)
		}
	}

	if len(videoIDs) > 0 {
		liked, err := l.likes.FilterLiked(ctx, likeevent.TargetVideo, userID, videoIDs)
		if err != nil {
			return nil, err
		}
		for i := range list {
			list[i].IsLiked = liked[list[i].Id]
		}
	}

	targets := make([]uint64, 0, len(authorIDs))
	for _, authorID := range authorIDs {
		if authorID != userID {
			targets = append(targets, authorID)
		}
	}
	if len(targets) > 0 {
		followed, err := l.follows.FilterFollowing(ctx, userID, targets)
		if err != nil {
			return nil, err
		}
		for i := range list {
			list[i].IsFollow = followed[list[i].AuthorID]
		}
	}
	return list, nil
}

func orderVideos(items []videorepo.Video, ids []uint64) []videorepo.Video {
	byID := make(map[uint64]videorepo.Video, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	ordered := make([]videorepo.Video, 0, len(ids))
	for _, id := range ids {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered
}
