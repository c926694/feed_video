package feed

import (
	"simple_tiktok/internal/modulekit"
	consumer2 "simple_tiktok/internal/mq/consumer"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/svc"
	"simple_tiktok/internal/service"
)

func RegisterConsumers(registrar modulekit.ConsumerRegistrar, ctx *svc.ServiceContext) error {
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	hotMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return err
	}
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, hotMQ)
	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return err
	}
	videoHotConsumer := consumer2.NewVideoHotConsumer(consumerChannel, videoRepo, feedService)
	if err := videoHotConsumer.Declare(event.VideoHotExchange, event.VideoHotExchangeType,
		event.VideoHotQueue, event.VideoHotRoutingKey); err != nil {
		return err
	}
	registrar.Add("feed.hot", func() error {
		return videoHotConsumer.Listen(event.VideoHotQueue, videoHotConsumer.HotUpdateHandler)
	})
	return nil
}
