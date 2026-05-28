package like

import (
	"simple_tiktok/internal/controller"
	"simple_tiktok/internal/modulekit"
	consumer2 "simple_tiktok/internal/mq/consumer"
	mysqlrepo "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	controller   *controller.LikeController
	likeConsumer *consumer2.LikeConsumer
	redis        *redis.Client
}

func NewModule(ctx modulekit.Context) (*Module, error) {
	serviceChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	likeService := service.NewLikeService(ctx.Redis, serviceChannel)

	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoRepo := mysqlrepo.NewVideoRepo(ctx.DB)
	commentRepo := mysqlrepo.NewCommentRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, mysqlrepo.NewUserRepo(ctx.DB), ctx.Redis, consumerChannel)
	likeConsumer := consumer2.NewLikeConsumer(consumerChannel, videoRepo, commentRepo, feedService)

	return &Module{
		controller:   controller.NewLikeController(likeService),
		likeConsumer: likeConsumer,
		redis:        ctx.Redis,
	}, nil
}
