package like

import (
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	likeService := NewService(ctx.Redis, ctx.Publisher)
	httpHandler := NewHTTPHandler(likeService)
	likeGroup := r.Group("likes")
	{
		likeGroup.POST("/video/switchLike/:id", ctx.Auth.Middleware(), httpHandler.LikeVideo)
		likeGroup.POST("/comment/switchLike/:id", ctx.Auth.Middleware(), httpHandler.LikeComment)
	}
	return r, nil
}
