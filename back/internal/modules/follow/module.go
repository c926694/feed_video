package follow

import (
	"simple_tiktok/internal/controller"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
	consumer2 "simple_tiktok/internal/mq/consumer"
	mysqlrepo "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type Module struct {
	controller     *controller.FollowController
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
		controller:     controller.NewFollowController(followService),
		followConsumer: followConsumer,
		redis:          ctx.Redis,
	}, nil
}

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	followGroup := r.Group("follows")
	{
		followGroup.POST("/switchFollow/:follower", middleware.JWTAuth(m.redis), m.controller.Follow)
	}
	return nil
}

func (m *Module) RegisterConsumers(registrar modulekit.ConsumerRegistrar) error {
	if err := m.followConsumer.Declare(event.FollowExchange, event.FollowExchangeType,
		event.FollowQueue, event.FollowRoutingKey); err != nil {
		return err
	}
	registrar.Add("follow.switch", func() error {
		return m.followConsumer.ListenFollowConsumer(event.FollowQueue, m.followConsumer.FollowHandler)
	})
	return nil
}
