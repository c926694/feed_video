package feed

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/platform/kafka"
	"simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"
)

func RegisterConsumers(sub *kafka.Subscriber, ctx *svc.ServiceContext) error {
	videoRepo := mysql.NewVideoRepo(ctx.DB)
	userRepo := mysql.NewUserRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, ctx.Publisher, ctx.Upload)

	sub.Subscribe(kafka.TopicVideoHot, func(handlerCtx context.Context, payload []byte) error {
		return handleVideoHot(handlerCtx, payload, videoRepo, feedService)
	})
	return nil
}

// handleVideoHot 消费热度事件，把分值累加到对应的分钟桶
func handleVideoHot(ctx context.Context, payload []byte, videoRepo *mysql.VideoRepo, feedService *service.FeedService) error {
	var videoHotEvent event.VideoHotEvent
	if err := json.Unmarshal(payload, &videoHotEvent); err != nil {
		return kafka.Permanent(err)
	}
	if videoHotEvent.VideoId == 0 {
		return kafka.Permanent(errors.New("热度事件里没有 videoId"))
	}
	if videoHotEvent.ScoreDelta == 0 {
		return nil
	}

	if _, err := videoRepo.GetVideoById(videoHotEvent.VideoId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 视频已经被删除，热度不再需要维护
			return kafka.Permanent(err)
		}
		return err
	}

	minute := time.Now()
	if videoHotEvent.MinuteStamp > 0 {
		minute = time.Unix(videoHotEvent.MinuteStamp, 0)
	}
	return feedService.IncrementHotScoreByMinute(videoHotEvent.VideoId, videoHotEvent.ScoreDelta, minute)
}
