package like

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm"

	commentrepo "simple_tiktok/internal/modules/comment/repo"
	favoriterepo "simple_tiktok/internal/modules/favorite/repo"
	followrepo "simple_tiktok/internal/modules/follow/repo"
	"simple_tiktok/internal/modules/like/event"
	"simple_tiktok/internal/modules/like/repo"
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

// Logic 点赞模块的业务逻辑
type Logic struct {
	repo      *repo.Repo
	videos    *videorepo.Repo
	comments  *commentrepo.Repo
	favorites *favoriterepo.Repo
	follows   *followrepo.Repo
	producer  *producer.Producer
	uploader  *upload.Uploader
}

// SetVideoLike 把点赞状态设置成目标态，幂等：重复设置同一目标态不发事件
func (l *Logic) SetVideoLike(ctx context.Context, videoID uint64, userID uint64, active bool) (bool, error) {
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
	return l.setLike(ctx, event.TargetVideo, videoID, userID, active)
}

// SetCommentLike 把评论点赞状态设置成目标态，幂等
func (l *Logic) SetCommentLike(ctx context.Context, commentID uint64, userID uint64, active bool) (bool, error) {
	if _, err := l.comments.GetByID(ctx, commentID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, httpx.New(httpx.CodeNotFound, "评论不存在")
		}
		return false, err
	}
	return l.setLike(ctx, event.TargetComment, commentID, userID, active)
}

// setLike 把点赞状态设置成目标态。状态没变时幂等返回，不发事件；
// 状态变了才发事件，发布失败时把状态写回之前的状态
func (l *Logic) setLike(ctx context.Context, target string, targetID uint64, userID uint64, active bool) (bool, error) {
	changed, err := l.repo.Set(ctx, target, targetID, userID, active)
	if err != nil {
		return false, err
	}
	if !changed {
		// 本来就是目标态，幂等返回，不重复发事件
		return active, nil
	}

	if err = l.producer.Publish(ctx, topic.LikeSwitched, strconv.FormatUint(targetID, 10), event.SwitchedEvent{
		Target:     target,
		TargetID:   targetID,
		Liked:      active,
		Operator:   userID,
		OccurredAt: time.Now().UnixMilli(),
	}); err != nil {
		if _, rollbackErr := l.repo.Set(ctx, target, targetID, userID, !active); rollbackErr != nil {
			return false, httpx.New(httpx.CodeInternal, fmt.Sprintf("点赞状态回滚失败: %v", rollbackErr))
		}
		return false, err
	}
	return active, nil
}

// ListMyLikedVideos 我的点赞列表：按点赞时间倒序双字段游标分页
func (l *Logic) ListMyLikedVideos(ctx context.Context, userID uint64, lastCreatedAt int64, lastID uint64, limit uint64) (*ListRes, error) {
	limit = normalizeLimit(limit)
	var cursor time.Time
	if lastID > 0 {
		cursor = time.UnixMilli(lastCreatedAt)
	}

	items, err := l.repo.ListVideosByUser(ctx, userID, cursor, lastID, int(limit)+1)
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
			IsLiked:       true,
			CreatedAt:     item.CreatedAt,
		})
		videoIDs = append(videoIDs, item.ID)
		if item.AuthorID != 0 && !seen[item.AuthorID] {
			seen[item.AuthorID] = true
			authorIDs = append(authorIDs, item.AuthorID)
		}
	}

	if err = l.fillFavorited(ctx, list, videoIDs, userID); err != nil {
		return nil, err
	}
	if err = l.fillFollowed(ctx, list, authorIDs, userID); err != nil {
		return nil, err
	}

	result := &ListRes{List: list, HasMore: hasMore}
	if len(items) > 0 {
		last := items[len(items)-1]
		result.LastCreatedAt = last.LikedAt.UnixMilli()
		result.LastID = last.LikeID
	}
	return result, nil
}

// fillFavorited 批量补当前用户对这批视频的收藏状态
func (l *Logic) fillFavorited(ctx context.Context, list []ItemRes, videoIDs []uint64, userID uint64) error {
	if len(videoIDs) == 0 {
		return nil
	}
	favorited, err := l.favorites.FilterFavorited(ctx, userID, videoIDs)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].IsFavorited = favorited[list[i].Id]
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

// HandleSwitched 订阅点赞切换事件，异步维护 user_like 表
func (l *Logic) HandleSwitched(ctx context.Context, payload []byte) error {
	var switched event.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Operator == 0 || switched.TargetID == 0 {
		return consumer.Permanent(errors.New("点赞事件里缺少用户或目标 ID"))
	}
	if switched.Target != event.TargetVideo && switched.Target != event.TargetComment {
		return consumer.Permanent(errors.New("点赞事件里 target 取值非法"))
	}

	var err error
	if switched.Liked {
		err = l.repo.Create(ctx, switched.Operator, switched.Target, switched.TargetID)
	} else {
		err = l.repo.Delete(ctx, switched.Operator, switched.Target, switched.TargetID)
	}
	if err != nil {
		slog.Error("维护点赞关系失败", "target", switched.Target, "target_id", switched.TargetID, "error", err)
		return err
	}
	return nil
}

// HandleVideoDeleted 视频被删除后清理它的全部点赞
func (l *Logic) HandleVideoDeleted(ctx context.Context, payload []byte) error {
	var deleted videoevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.VideoID == 0 {
		return consumer.Permanent(errors.New("删除视频事件里没有 videoId"))
	}
	if err := l.repo.DeleteByTarget(ctx, event.TargetVideo, deleted.VideoID); err != nil {
		slog.Error("清理视频点赞失败", "video_id", deleted.VideoID, "error", err)
		return err
	}
	if err := l.repo.DeleteTargetSet(ctx, event.TargetVideo, deleted.VideoID); err != nil {
		slog.Error("清理视频点赞集合失败", "video_id", deleted.VideoID, "error", err)
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
