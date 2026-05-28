package like

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
	likeConsumer := consumer2.NewLikeConsumer(consumerChannel, videoRepo, commentRepo, ctx.Redis)

	return &Module{
		controller:   controller.NewLikeController(likeService),
		likeConsumer: likeConsumer,
		redis:        ctx.Redis,
	}, nil
}

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	likeGroup := r.Group("likes")
	{
		likeGroup.POST("/video/switchLike/:id", middleware.JWTAuth(m.redis), m.controller.LikeVideo)
		likeGroup.POST("/comment/switchLike/:id", middleware.JWTAuth(m.redis), m.controller.LikeComment)
	}
	return nil
}

func (m *Module) RegisterConsumers(registrar modulekit.ConsumerRegistrar) error {
	if err := m.likeConsumer.Declare(event.LikeVideoExchange, event.LikeVideoExchangeType,
		event.LikeVideoQueue, event.LikeVideoRoutingKey); err != nil {
		return err
	}
	if err := m.likeConsumer.Declare(event.LikeCommentExchange, event.LikeCommentExchangeType,
		event.LikeCommentQueue, event.LikeCommentRoutingKey); err != nil {
		return err
	}
	registrar.Add("like.video", func() error {
		return m.likeConsumer.ListenLikeConsumer(event.LikeVideoQueue, m.likeConsumer.LikeVideoHandler)
	})
	registrar.Add("like.comment", func() error {
		return m.likeConsumer.ListenLikeConsumer(event.LikeCommentQueue, m.likeConsumer.LikeCommentHandler)
	})
	return nil
}
