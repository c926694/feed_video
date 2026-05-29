package like

import (
	consumer2 "simple_tiktok/internal/mq/consumer"
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"
)

func RegisterConsumers(registrar modulekit.ConsumerRegistrar, ctx *svc.ServiceContext) error {
	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return err
	}
	videoRepo := NewVideoRepo(ctx.DB)
	commentRepo := NewCommentRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, NewUserRepo(ctx.DB), ctx.Redis, consumerChannel)
	likeConsumer := consumer2.NewLikeConsumer(consumerChannel, videoRepo, commentRepo, feedService)
	if err := likeConsumer.Declare(event.LikeVideoExchange, event.LikeVideoExchangeType,
		event.LikeVideoQueue, event.LikeVideoRoutingKey); err != nil {
		return err
	}
	if err := likeConsumer.Declare(event.LikeCommentExchange, event.LikeCommentExchangeType,
		event.LikeCommentQueue, event.LikeCommentRoutingKey); err != nil {
		return err
	}
	registrar.Add("like.video", func() error {
		return likeConsumer.ListenLikeConsumer(event.LikeVideoQueue, likeConsumer.LikeVideoHandler)
	})
	registrar.Add("like.comment", func() error {
		return likeConsumer.ListenLikeConsumer(event.LikeCommentQueue, likeConsumer.LikeCommentHandler)
	})
	return nil
}
