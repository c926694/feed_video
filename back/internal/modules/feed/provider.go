package feed

import (
	"simple_tiktok/internal/modulekit"
	consumer2 "simple_tiktok/internal/mq/consumer"
	mysqlrepo "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	videoHotConsumer *consumer2.VideoHotConsumer
	feedHandler      *HTTPHandler
	redis            *redis.Client
}

func NewModule(ctx modulekit.Context) (*Module, error) {
	videoRepo := mysqlrepo.NewVideoRepo(ctx.DB)
	userRepo := mysqlrepo.NewUserRepo(ctx.DB)
	commentRepo := mysqlrepo.NewCommentRepo(ctx.DB)
	serviceChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoService := service.NewVideoService(videoRepo, userRepo, ctx.Redis, serviceChannel, commentRepo)

	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoHotConsumer := consumer2.NewVideoHotConsumer(consumerChannel, videoRepo, videoService)
	return &Module{
		videoHotConsumer: videoHotConsumer,
		feedHandler:      NewHTTPHandler(videoService),
		redis:            ctx.Redis,
	}, nil
}
