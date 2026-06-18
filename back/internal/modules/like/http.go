package like

import (
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
	kafkaproducer "simple_tiktok/internal/mq/kafka/producer"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	likeVideoWriter := kafkaproducer.NewProducer(ctx.KafkaBrokers, kafkaproducer.LikeVideoTopic)
	likeCommentWriter := kafkaproducer.NewProducer(ctx.KafkaBrokers, kafkaproducer.LikeCommentTopic)
	likeService := NewService(ctx.Redis, likeVideoWriter.Writer, likeCommentWriter.Writer)
	httpHandler := NewHTTPHandler(likeService)
	likeGroup := r.Group("likes")
	{
		likeGroup.POST("/video/switchLike/:id", middleware.JWTAuth(ctx.Redis), httpHandler.LikeVideo)
		likeGroup.POST("/comment/switchLike/:id", middleware.JWTAuth(ctx.Redis), httpHandler.LikeComment)
	}
	return r, nil
}
