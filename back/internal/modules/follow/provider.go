package follow

import (
	"simple_tiktok/internal/modulekit"
	consumer2 "simple_tiktok/internal/mq/consumer"
	mysqlrepo "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	httpHandler    *HTTPHandler
	followConsumer *consumer2.FollowConsumer
	redis          *redis.Client
}

func NewModule(ctx modulekit.Context) (*Module, error) {
	serviceChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	followService := service.NewFollowService(ctx.Redis, serviceChannel)

	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	followRepo := mysqlrepo.NewFollowRepo(ctx.DB)
	userRepo := mysqlrepo.NewUserRepo(ctx.DB)
	followConsumer := consumer2.NewFollowConsumer(consumerChannel, followRepo, userRepo)

	return &Module{
		httpHandler:    NewHTTPHandler(followService),
		followConsumer: followConsumer,
		redis:          ctx.Redis,
	}, nil
}
