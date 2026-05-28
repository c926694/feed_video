package video

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
	controller    *controller.VideoController
	videoConsumer *consumer2.VideoConsumer
	redis         *redis.Client
}

func NewModule(ctx modulekit.Context) (*Module, error) {
	videoRepo := mysqlrepo.NewVideoRepo(ctx.DB)
	userRepo := mysqlrepo.NewUserRepo(ctx.DB)
	commentRepo := mysqlrepo.NewCommentRepo(ctx.DB)

	videoMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoService := service.NewVideoService(videoRepo, userRepo, ctx.Redis, videoMQ, commentRepo)
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

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	videoGroup := r.Group("videos")
	{
		videoGroup.POST("/create", middleware.JWTAuth(m.redis), m.controller.CreateVideo)
		videoGroup.DELETE("/:id", middleware.JWTAuth(m.redis), m.controller.DeleteVideos)
		videoGroup.GET("/me", middleware.JWTAuth(m.redis), m.controller.GetMyVideos)
		videoGroup.GET("/feed", middleware.JWTAuth(m.redis), m.controller.GetFeedVideos)
		videoGroup.GET("/feed/hot", middleware.JWTAuth(m.redis), m.controller.GetFeedHotVideos)
		videoGroup.GET("/feed/follow", middleware.JWTAuth(m.redis), m.controller.GetFollowFeedVideos)
		videoGroup.GET("/:id", middleware.JWTAuth(m.redis), m.controller.GetVideoInfo)
	}
	return nil
}

func (m *Module) RegisterConsumers(registrar modulekit.ConsumerRegistrar) error {
	if err := m.videoConsumer.Declare(event.DeleteVideoExchange, event.DeleteVideoExchangeType,
		event.DeleteVideoQueue, event.DeleteVideoRoutingKey); err != nil {
		return err
	}
	registrar.Add("video.delete", func() error {
		return m.videoConsumer.ListenVideoConsumer(event.DeleteVideoQueue, m.videoConsumer.DeleteVideoHandler)
	})
	return nil
}
