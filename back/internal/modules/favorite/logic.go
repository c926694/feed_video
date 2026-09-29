package favorite

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm"

	favoriteevent "simple_tiktok/internal/modules/favorite/event"
	favoriterepo "simple_tiktok/internal/modules/favorite/repo"
	likeevent "simple_tiktok/internal/modules/like/event"
	likerepo "simple_tiktok/internal/modules/like/repo"
	followrepo "simple_tiktok/internal/modules/follow/repo"
	videoevent "simple_tiktok/internal/modules/video/event"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/platform/upload"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

// Logic 收藏模块的业务逻辑。收藏集合在 Redis，关系表由事件异步维护
type Logic struct {
	repo     *favoriterepo.Repo
	videos   *videorepo.Repo
	likes    *likerepo.Repo
	follows  *followrepo.Repo
	producer *producer.Producer
	uploader *upload.Uploader
}

// SetVideoFavorite 把收藏状态设置成目标态，幂等：重复设置同一目标态不发事件
func (l *Logic) SetVideoFavorite(ctx context.Context, videoID uint64, userID uint64, active bool) (bool, error) {
	video, err := l.videos.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, httpx.New(httpx.CodeNotFound, "视频不存在")
		}
		return false, err
	}
	if video.Status != videorepo.StatusPublished {
		return false, httpx.New(httpx.CodeNotFound, "视频不存在")
	}
	return l.setFavorite(ctx, videoID, userID, active)
}

// setFavorite 把收藏状态设置成目标态。状态没变时幂等返回，不发事件；
// 状态变了才发事件，发布失败时把状态写回之前的状态
func (l *Logic) setFavorite(ctx context.Context, videoID uint64, userID uint64, active bool) (bool, error) {
	changed, err := l.repo.Set(ctx, videoID, userID, active)
	if err != nil {
		return false, err
	}
	if !changed {
		// 本来就是目标态，幂等返回，不重复发事件
		return active, nil
	}

	if err = l.producer.Publish(ctx, topic.FavoriteSwitched, strconv.FormatUint(videoID, 10), favoriteevent.SwitchedEvent{
		VideoID:    videoID,
		UserID:     userID,
		Favorited:  active,
		OccurredAt: time.Now().UnixMilli(),
	}); err != nil {
		if _, rollbackErr := l.repo.Set(ctx, videoID, userID, !active); rollbackErr != nil {
			return false, httpx.New(httpx.CodeInternal, "收藏状态回滚失败")
		}
		return false, err
	}
	return active, nil
}

// ListMyFavorites 我的收藏列表：按收藏时间倒序双字段游标分页
func (l *Logic) ListMyFavorites(ctx context.Context, userID uint64, lastCreatedAt int64, lastID uint64, limit uint64) (*ListRes, error) {
	limit = normalizeLimit(limit)
	var cursor time.Time
	if lastID > 0 {
		cursor = time.UnixMilli(lastCreatedAt)
	}

	items, err := l.repo.ListByUser(ctx, userID, cursor, lastID, int(limit)+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(items) > int(limit)
	if hasMore {
		items = items[:limit]
	}

	list := make([]ItemRes, 0, len(items))
	videoIDs := make([]uint64, 0, len(items))
	authorIDs := make([]uint64, 0, len(items))
	seen := make(map[uint64]bool, len(items))
	for _, item := range items {
		list = append(list, ItemRes{
			Id:            item.ID,
			AuthorID:      item.AuthorID,
			AuthorName:    item.AuthorName,
			AuthorAvatar:  l.uploader.URL(item.AuthorAvatar),
			Title:         item.Title,
			Description:   item.Description,
			CoverURL:      l.uploader.URL(item.CoverURL),
			PlayURL:       l.uploader.URL(item.PlayURL),
			CommentCount:  item.CommentCount,
			LikeCount:     item.LikeCount,
			FavoriteCount: item.FavoriteCount,
			IsFavorited:   true,
			CreatedAt:     item.CreatedAt,
		})
		videoIDs = append(videoIDs, item.ID)
		if item.AuthorID != 0 && !seen[item.AuthorID] {
			seen[item.AuthorID] = true
			authorIDs = append(authorIDs, item.AuthorID)
		}
	}

	if err = l.fillLiked(ctx, list, videoIDs, userID); err != nil {
		return nil, err
	}
	if err = l.fillFollowed(ctx, list, authorIDs, userID); err != nil {
		return nil, err
	}

	result := &ListRes{List: list, HasMore: hasMore}
	if len(items) > 0 {
		last := items[len(items)-1]
		result.LastCreatedAt = last.FavoritedAt.UnixMilli()
		result.LastID = last.FavoriteID
	}
	return result, nil
}

// fillLiked 批量补当前用户对这批视频的点赞状态
func (l *Logic) fillLiked(ctx context.Context, list []ItemRes, videoIDs []uint64, userID uint64) error {
	if len(videoIDs) == 0 {
		return nil
	}
	liked, err := l.likes.FilterLiked(ctx, likeevent.TargetVideo, userID, videoIDs)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].IsLiked = liked[list[i].Id]
	}
	return nil
}

// fillFollowed 批量补当前用户对作者的关注状态
func (l *Logic) fillFollowed(ctx context.Context, list []ItemRes, authorIDs []uint64, userID uint64) error {
	if len(authorIDs) == 0 {
		return nil
	}
	followed, err := l.follows.FilterFollowing(ctx, userID, authorIDs)
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].AuthorID == 0 || list[i].AuthorID == userID {
			list[i].IsFollow = false
			continue
		}
		list[i].IsFollow = followed[list[i].AuthorID]
	}
	return nil
}

// HandleSwitched 订阅自己的事件，异步维护 user_favorite 表
func (l *Logic) HandleSwitched(ctx context.Context, payload []byte) error {
	var switched favoriteevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.UserID == 0 || switched.VideoID == 0 {
		return consumer.Permanent(errors.New("收藏事件里缺少用户或视频 ID"))
	}

	var err error
	if switched.Favorited {
		err = l.repo.Create(ctx, switched.UserID, switched.VideoID)
	} else {
		err = l.repo.Delete(ctx, switched.UserID, switched.VideoID)
	}
	if err != nil {
		slog.Error("维护收藏关系失败", "video_id", switched.VideoID, "error", err)
		return err
	}
	return nil
}

// HandleVideoDeleted 视频被删除后清理它的全部收藏
func (l *Logic) HandleVideoDeleted(ctx context.Context, payload []byte) error {
	var deleted videoevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.VideoID == 0 {
		return consumer.Permanent(errors.New("删除视频事件里没有 videoId"))
	}
	if err := l.repo.DeleteByVideo(ctx, deleted.VideoID); err != nil {
		slog.Error("清理视频收藏失败", "video_id", deleted.VideoID, "error", err)
		return err
	}
	if err := l.repo.DeleteTargetSet(ctx, deleted.VideoID); err != nil {
		slog.Error("清理视频收藏集合失败", "video_id", deleted.VideoID, "error", err)
		return err
	}
	return nil
}

func normalizeLimit(limit uint64) uint64 {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}
