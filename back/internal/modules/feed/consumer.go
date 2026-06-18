package feed

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
	"time"

	"github.com/segmentio/kafka-go"
)

func RegisterConsumers(registrar modulekit.ConsumerRegistrar, ctx *svc.ServiceContext) error {
	videoRepo := mysql.NewVideoRepo(ctx.DB)
	userRepo := mysql.NewUserRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, ctx.KafkaBrokers)

	videoHotConsumer := consumer2.NewConsumer(ctx.KafkaBrokers, event.VideoHotTopic, "feed-hot-group")

	registrar.Add("feed.hot", func() error {
		return videoHotConsumer.Consume(context.Background(), func(ctx context.Context, msg kafka.Message) error {
			return handleVideoHot(msg, videoRepo, feedService)
		})
	})
	return nil
}

func handleVideoHot(msg kafka.Message, videoRepo *mysql.VideoRepo, feedService *service.FeedService) error {
	var e event.VideoHotEvent
	if err := json.Unmarshal(msg.Value, &e); err != nil {
		log.Println(err)
		return err
	}

	videoId := e.VideoId
	if e.ScoreDelta == 0 {
		return nil
	}
	_, err := videoRepo.GetVideoById(videoId)
	if err != nil {
		log.Println(err)
		return nil
	}

	minute := time.Now()
	if e.MinuteStamp > 0 {
		minute = time.Unix(e.MinuteStamp, 0)
	}
	if err := feedService.IncrementHotScoreByMinute(videoId, e.ScoreDelta, minute); err != nil {
		log.Println(err)
		return err
	}

	return nil
}
