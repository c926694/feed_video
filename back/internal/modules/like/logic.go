package like

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"gorm.io/gorm"

	commentevent "simple_tiktok/internal/modules/comment/event"
	commentrepo "simple_tiktok/internal/modules/comment/repo"
	"simple_tiktok/internal/modules/like/event"
	"simple_tiktok/internal/modules/like/repo"
	videoevent "simple_tiktok/internal/modules/video/event"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
)

// Logic 点赞模块的业务逻辑
type Logic struct {
	repo     *repo.Repo
	videos   *videorepo.Repo
	comments *commentrepo.Repo
	producer *producer.Producer
}

func (l *Logic) SwitchVideoLike(ctx context.Context, videoID uint64, userID uint64) (bool, error) {
	if _, err := l.videos.GetByID(ctx, videoID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, httpx.New(httpx.CodeNotFound, "视频不存在")
		}
		return false, err
	}
	return l.switchLike(ctx, event.TargetVideo, videoID, userID)
}

func (l *Logic) SwitchCommentLike(ctx context.Context, commentID uint64, userID uint64) (bool, error) {
	if _, err := l.comments.GetByID(ctx, commentID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, httpx.New(httpx.CodeNotFound, "评论不存在")
		}
		return false, err
	}
	return l.switchLike(ctx, event.TargetComment, commentID, userID)
}

func (l *Logic) switchLike(ctx context.Context, target string, targetID uint64, userID uint64) (bool, error) {
	liked, err := l.repo.Switch(ctx, target, targetID, userID)
	if err != nil {
		return false, err
	}

	err = l.producer.Publish(ctx, topic.LikeSwitched, strconv.FormatUint(targetID, 10), event.SwitchedEvent{
		Target:   target,
		TargetID: targetID,
		Liked:    liked,
		Operator: userID,
	})
	if err != nil {
		if rollbackErr := l.repo.Reset(ctx, target, targetID, userID, !liked); rollbackErr != nil {
			return false, httpx.New(httpx.CodeInternal, fmt.Sprintf("点赞状态回滚失败: %v", rollbackErr))
		}
		return false, err
	}
	return liked, nil
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

// HandleCommentDeleted 评论被删除后清理它的全部点赞
func (l *Logic) HandleCommentDeleted(ctx context.Context, payload []byte) error {
	var deleted commentevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.CommentID == 0 {
		return consumer.Permanent(errors.New("删除评论事件里没有 commentId"))
	}
	if err := l.repo.DeleteByTarget(ctx, event.TargetComment, deleted.CommentID); err != nil {
		slog.Error("清理评论点赞失败", "comment_id", deleted.CommentID, "error", err)
		return err
	}
	if err := l.repo.DeleteTargetSet(ctx, event.TargetComment, deleted.CommentID); err != nil {
		slog.Error("清理评论点赞集合失败", "comment_id", deleted.CommentID, "error", err)
		return err
	}
	return nil
}
