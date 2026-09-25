package follow

import (
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	followService := NewService(ctx.Redis, ctx.Publisher)
	httpHandler := NewHTTPHandler(followService)
	followGroup := r.Group("follows")
	{
		followGroup.POST("/switchFollow/:follower", ctx.Auth.Middleware(), httpHandler.Follow)
	}
	return r, nil
}
