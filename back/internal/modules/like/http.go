package like

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
	likeService := NewService(ctx.Redis, serviceChannel)
	httpHandler := NewHTTPHandler(likeService)
	likeGroup := r.Group("likes")
	{
		likeGroup.POST("/video/switchLike/:id", middleware.JWTAuth(ctx.Redis), httpHandler.LikeVideo)
		likeGroup.POST("/comment/switchLike/:id", middleware.JWTAuth(ctx.Redis), httpHandler.LikeComment)
	}
	return r, nil
}
