package video

import (
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
	kafkaproducer "simple_tiktok/internal/mq/kafka/producer"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	commentRepo := NewCommentRepo(ctx.DB)
	videoHotProducer := kafkaproducer.NewProducer(ctx.KafkaBrokers, kafkaproducer.VideoHotTopic)
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, ctx.KafkaBrokers)

	deleteProducer := kafkaproducer.NewProducer(ctx.KafkaBrokers, kafkaproducer.DeleteVideoTopic)
	videoService := NewService(videoRepo, userRepo, ctx.Redis, deleteProducer.Writer, commentRepo, feedService)
	httpHandler := NewHTTPHandler(videoService)
	videoGroup := r.Group("videos")
	{
		videoGroup.POST("/create", middleware.JWTAuth(ctx.Redis), httpHandler.CreateVideo)
		videoGroup.DELETE("/:id", middleware.JWTAuth(ctx.Redis), httpHandler.DeleteVideos)
		videoGroup.GET("/me", middleware.JWTAuth(ctx.Redis), httpHandler.GetMyVideos)
		videoGroup.GET("/:id", middleware.JWTAuth(ctx.Redis), httpHandler.GetVideoInfo)
	}
	_ = videoHotProducer
	return r, nil
}
