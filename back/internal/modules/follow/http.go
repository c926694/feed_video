package follow

import (
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	serviceChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	followService := NewService(ctx.Redis, serviceChannel)
	httpHandler := NewHTTPHandler(followService)
	followGroup := r.Group("follows")
	{
		followGroup.POST("/switchFollow/:follower", middleware.JWTAuth(ctx.Redis), httpHandler.Follow)
	}
	return r, nil
}
