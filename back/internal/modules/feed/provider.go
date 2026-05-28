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
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis)

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
