package like

import (
	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/modules/like/repo"
	"simple_tiktok/internal/svc"
)

// NewLogic 装配本模块的业务层，依赖全部来自进程级的 ServiceContext
func NewLogic(ctx *svc.ServiceContext) *Logic {
	return &Logic{
		repo:     repo.New(ctx.Redis),
		producer: ctx.Producer,
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewLogic(ctx))
	group := r.Group("likes")
	{
		group.POST("/video/switchLike/:id", ctx.Auth.Middleware(), controller.LikeVideo)
		group.POST("/comment/switchLike/:id", ctx.Auth.Middleware(), controller.LikeComment)
	}
	return r, nil
}
