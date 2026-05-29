package follow

import (
	consumer2 "simple_tiktok/internal/mq/consumer"
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/svc"
)

func RegisterConsumers(registrar modulekit.ConsumerRegistrar, ctx *svc.ServiceContext) error {
	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return err
	}
	followRepo := NewFollowRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	followConsumer := consumer2.NewFollowConsumer(consumerChannel, followRepo, userRepo)
	if err := followConsumer.Declare(event.FollowExchange, event.FollowExchangeType,
		event.FollowQueue, event.FollowRoutingKey); err != nil {
		return err
	}
	registrar.Add("follow.switch", func() error {
		return followConsumer.ListenFollowConsumer(event.FollowQueue, followConsumer.FollowHandler)
	})
	return nil
}
