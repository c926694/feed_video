package like

import (
	consumer2 "simple_tiktok/internal/mq/consumer"
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	httpHandler  *HTTPHandler
	likeConsumer *consumer2.LikeConsumer
	redis        *redis.Client
}

func NewModule(ctx *svc.ServiceContext) (*Module, error) {
	serviceChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	likeService := NewService(ctx.Redis, serviceChannel)

	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoRepo := NewVideoRepo(ctx.DB)
	commentRepo := NewCommentRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, NewUserRepo(ctx.DB), ctx.Redis, consumerChannel)
	likeConsumer := consumer2.NewLikeConsumer(consumerChannel, videoRepo, commentRepo, feedService)

	return &Module{
		httpHandler:  NewHTTPHandler(likeService),
		likeConsumer: likeConsumer,
		redis:        ctx.Redis,
	}, nil
}
