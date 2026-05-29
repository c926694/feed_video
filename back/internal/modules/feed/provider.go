package feed

import (
	consumer2 "simple_tiktok/internal/mq/consumer"
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	videoHotConsumer *consumer2.VideoHotConsumer
	feedHandler      *HTTPHandler
	redis            *redis.Client
}

func NewModule(ctx *svc.ServiceContext) (*Module, error) {
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	hotMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, hotMQ)

	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoHotConsumer := consumer2.NewVideoHotConsumer(consumerChannel, videoRepo, feedService)
	return &Module{
		videoHotConsumer: videoHotConsumer,
		feedHandler:      NewHTTPHandler(feedService),
		redis:            ctx.Redis,
	}, nil
}
