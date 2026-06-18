package follow

import (
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
	kafkaproducer "simple_tiktok/internal/mq/kafka/producer"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	followWriter := kafkaproducer.NewProducer(ctx.KafkaBrokers, kafkaproducer.FollowTopic)
	followService := NewService(ctx.Redis, followWriter.Writer)
	httpHandler := NewHTTPHandler(followService)
	followGroup := r.Group("follows")
	{
		followGroup.POST("/switchFollow/:follower", middleware.JWTAuth(ctx.Redis), httpHandler.Follow)
	}
	return r, nil
}
