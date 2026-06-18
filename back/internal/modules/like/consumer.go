package like

import (
	"context"
	"encoding/json"
	"log"
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
	consumer2 "simple_tiktok/internal/mq/kafka/consumer"
	"simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"

	"github.com/segmentio/kafka-go"
)

func RegisterConsumers(registrar modulekit.ConsumerRegistrar, ctx *svc.ServiceContext) error {
	videoRepo := mysql.NewVideoRepo(ctx.DB)
	commentRepo := mysql.NewCommentRepo(ctx.DB)
	userRepo := mysql.NewUserRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, ctx.KafkaBrokers)

	likeVideoConsumer := consumer2.NewConsumer(ctx.KafkaBrokers, event.LikeVideoTopic, "like-video-group")
	likeCommentConsumer := consumer2.NewConsumer(ctx.KafkaBrokers, event.LikeCommentTopic, "like-comment-group")

	registrar.Add("like.video", func() error {
		return likeVideoConsumer.Consume(context.Background(), func(ctx context.Context, msg kafka.Message) error {
			return handleLikeVideo(msg, videoRepo, feedService)
		})
	})
	registrar.Add("like.comment", func() error {
		return likeCommentConsumer.Consume(context.Background(), func(ctx context.Context, msg kafka.Message) error {
			return handleLikeComment(msg, commentRepo)
		})
	})
	return nil
}

func handleLikeVideo(msg kafka.Message, videoRepo *mysql.VideoRepo, feedService *service.FeedService) error {
	var videoEvent event.LikeVideoEvent
	if err := json.Unmarshal(msg.Value, &videoEvent); err != nil {
		log.Println(err)
		return err
	}
	videoId := videoEvent.VideoId
	eventType := videoEvent.EventType
	hotDelta := 0.0
	switch eventType {
	case event.Like:
		if err := videoRepo.IncVideoLikeCount(videoId); err != nil {
			log.Println(err)
			return err
		}
		hotDelta = 2
	case event.Dislike:
		if err := videoRepo.DecVideoDislikeCount(videoId); err != nil {
			log.Println(err)
			return err
		}
		hotDelta = -2
	default:
		log.Printf("unsupported like event type: %s", eventType)
		return nil
	}
	if feedService != nil {
		feedService.MustInvalidateVideoInfoCache(videoId)
		if err := feedService.PublishVideoHotEvent(videoId, hotDelta); err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func handleLikeComment(msg kafka.Message, commentRepo *mysql.CommentRepo) error {
	var commentEvent event.LikeCommentEvent
	if err := json.Unmarshal(msg.Value, &commentEvent); err != nil {
		log.Println(err)
		return err
	}

	commentId := commentEvent.CommentId
	switch commentEvent.EventType {
	case event.Like:
		if err := commentRepo.IncCommentLikeCount(commentId); err != nil {
			log.Println(err)
			return err
		}
	case event.Dislike:
		if err := commentRepo.DecCommentLikeCount(commentId); err != nil {
			log.Println(err)
			return err
		}
	default:
		log.Printf("unsupported comment like event type: %s", commentEvent.EventType)
		return nil
	}
	return nil
}
