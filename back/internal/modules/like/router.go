package like

import (
	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/svc"
)

func RegisterHTTP(r *gin.Engine, model *Model, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(model)
	group := r.Group("likes")
	{
		group.POST("/video/switchLike/:id", ctx.Auth.Middleware(), controller.LikeVideo)
		group.POST("/comment/switchLike/:id", ctx.Auth.Middleware(), controller.LikeComment)
	}
	return r, nil
}
