package like

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/platform/kafka"
	"simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"
)

func RegisterConsumers(sub *kafka.Subscriber, ctx *svc.ServiceContext) error {
	videoRepo := mysql.NewVideoRepo(ctx.DB)
	commentRepo := mysql.NewCommentRepo(ctx.DB)
	userRepo := mysql.NewUserRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, ctx.Publisher, ctx.Upload)

	sub.Subscribe(kafka.TopicLikeVideo, func(handlerCtx context.Context, payload []byte) error {
		return handleLikeVideo(handlerCtx, payload, videoRepo, feedService)
	})
	sub.Subscribe(kafka.TopicLikeComment, func(handlerCtx context.Context, payload []byte) error {
		return handleLikeComment(handlerCtx, payload, commentRepo)
	})
	return nil
}

// handleLikeVideo 消费视频点赞事件，更新点赞计数并调整热度
func handleLikeVideo(ctx context.Context, payload []byte, videoRepo *mysql.VideoRepo, feedService *service.FeedService) error {
	var videoEvent event.LikeVideoEvent
	if err := json.Unmarshal(payload, &videoEvent); err != nil {
		return kafka.Permanent(err)
	}
	if videoEvent.VideoId == 0 {
		return kafka.Permanent(errors.New("点赞事件里没有 videoId"))
	}

	hotDelta := 0.0
	switch videoEvent.EventType {
	case event.Like:
		if err := videoRepo.IncVideoLikeCount(videoEvent.VideoId); err != nil {
			slog.Error("更新视频点赞数失败", "video_id", videoEvent.VideoId, "error", err)
			return err
		}
		hotDelta = 2
	case event.Dislike:
		if err := videoRepo.DecVideoDislikeCount(videoEvent.VideoId); err != nil {
			slog.Error("减少视频点赞数失败", "video_id", videoEvent.VideoId, "error", err)
			return err
		}
		hotDelta = -2
	default:
		slog.Warn("不支持的点赞事件类型", "event_type", videoEvent.EventType)
		return nil
	}

	if feedService != nil {
		feedService.MustInvalidateVideoInfoCache(videoEvent.VideoId)
		if err := feedService.PublishVideoHotEvent(videoEvent.VideoId, hotDelta); err != nil {
			slog.Error("发布热度事件失败", "video_id", videoEvent.VideoId, "error", err)
			return err
		}
	}
	return nil
}

// handleLikeComment 消费评论点赞事件，更新评论点赞数
func handleLikeComment(ctx context.Context, payload []byte, commentRepo *mysql.CommentRepo) error {
	var commentEvent event.LikeCommentEvent
	if err := json.Unmarshal(payload, &commentEvent); err != nil {
		return kafka.Permanent(err)
	}
	if commentEvent.CommentId == 0 {
		return kafka.Permanent(errors.New("评论点赞事件里没有 commentId"))
	}

	switch commentEvent.EventType {
	case event.Like:
		if err := commentRepo.IncCommentLikeCount(commentEvent.CommentId); err != nil {
			slog.Error("更新评论点赞数失败", "comment_id", commentEvent.CommentId, "error", err)
			return err
		}
	case event.Dislike:
		if err := commentRepo.DecCommentLikeCount(commentEvent.CommentId); err != nil {
			slog.Error("减少评论点赞数失败", "comment_id", commentEvent.CommentId, "error", err)
			return err
		}
	default:
		slog.Warn("不支持的评论点赞事件类型", "event_type", commentEvent.EventType)
		return nil
	}
	return nil
}
