package favorite

import (
	"github.com/gin-gonic/gin"

	favoriterepo "simple_tiktok/internal/modules/favorite/repo"
	followrepo "simple_tiktok/internal/modules/follow/repo"
	likerepo "simple_tiktok/internal/modules/like/repo"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

// Name 模块名，用作消费组的一部分，多个模块订阅同一个 topic 时互不争抢
const Name = "favorite"

// NewLogic 装配本模块的业务逻辑，依赖全部来自进程级的 ServiceContext
func NewLogic(ctx *svc.ServiceContext) *Logic {
	return &Logic{
		repo:     favoriterepo.New(ctx.DB, ctx.Redis),
		videos:   videorepo.New(ctx.DB, ctx.Redis),
		likes:    likerepo.New(ctx.DB, ctx.Redis),
		follows:  followrepo.New(ctx.DB, ctx.Redis),
		producer: ctx.Producer,
		uploader: ctx.Upload,
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewLogic(ctx))
	group := r.Group("favorites")
	{
		group.POST("/video/:id", ctx.Auth.Middleware(), controller.Favorite)
		group.DELETE("/video/:id", ctx.Auth.Middleware(), controller.Unfavorite)
		group.GET("/me", ctx.Auth.Middleware(), controller.ListMine)
	}
	return r, nil
}

// RegisterConsumers 订阅收藏事件维护关系表，以及删除视频事件清理收藏
func RegisterConsumers(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
	moduleLogic := NewLogic(ctx)
	sub.Subscribe(topic.FavoriteSwitched, moduleLogic.HandleSwitched)
	sub.Subscribe(topic.VideoDeleted, moduleLogic.HandleVideoDeleted)
	return nil
}
