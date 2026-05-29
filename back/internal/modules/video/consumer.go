package video

import (
	"simple_tiktok/internal/modulekit"
	consumer2 "simple_tiktok/internal/mq/consumer"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/svc"
)

func RegisterConsumers(registrar modulekit.ConsumerRegistrar, ctx *svc.ServiceContext) error {
	videoRepo := NewVideoRepo(ctx.DB)
	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return err
	}
	videoConsumer := consumer2.NewVideoConsumer(consumerChannel, videoRepo)
	if err := videoConsumer.Declare(event.DeleteVideoExchange, event.DeleteVideoExchangeType,
		event.DeleteVideoQueue, event.DeleteVideoRoutingKey); err != nil {
		return err
	}
	registrar.Add("video.delete", func() error {
		return videoConsumer.ListenVideoConsumer(event.DeleteVideoQueue, videoConsumer.DeleteVideoHandler)
	})
	return nil
}
