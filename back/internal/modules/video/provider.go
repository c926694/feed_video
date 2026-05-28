package video

import (
	"simple_tiktok/internal/controller"
	"simple_tiktok/internal/modulekit"
	consumer2 "simple_tiktok/internal/mq/consumer"
	mysqlrepo "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	controller    *controller.VideoController
	videoConsumer *consumer2.VideoConsumer
	redis         *redis.Client
}

func NewModule(ctx modulekit.Context) (*Module, error) {
	videoRepo := mysqlrepo.NewVideoRepo(ctx.DB)
	userRepo := mysqlrepo.NewUserRepo(ctx.DB)
	commentRepo := mysqlrepo.NewCommentRepo(ctx.DB)
	hotMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, hotMQ)

	videoMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoService := service.NewVideoService(videoRepo, userRepo, ctx.Redis, videoMQ, commentRepo, feedService)
	videoController := controller.NewVideoController(videoService)

	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoConsumer := consumer2.NewVideoConsumer(consumerChannel, videoRepo)

	return &Module{
		controller:    videoController,
		videoConsumer: videoConsumer,
		redis:         ctx.Redis,
	}, nil
}
