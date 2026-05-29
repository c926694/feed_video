package follow

import (
	consumer2 "simple_tiktok/internal/mq/consumer"
	"simple_tiktok/internal/svc"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	httpHandler    *HTTPHandler
	followConsumer *consumer2.FollowConsumer
	redis          *redis.Client
}

func NewModule(ctx *svc.ServiceContext) (*Module, error) {
	serviceChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	followService := NewService(ctx.Redis, serviceChannel)

	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	followRepo := NewFollowRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	followConsumer := consumer2.NewFollowConsumer(consumerChannel, followRepo, userRepo)

	return &Module{
		httpHandler:    NewHTTPHandler(followService),
		followConsumer: followConsumer,
		redis:          ctx.Redis,
	}, nil
}
