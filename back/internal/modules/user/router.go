package user

import (
	"github.com/gin-gonic/gin"

	followrepo "simple_tiktok/internal/modules/follow/repo"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

// Name 模块名，用作消费组的一部分，多个模块订阅同一个 topic 时互不争抢
const Name = "user"

// NewLogic 装配本模块的业务层，依赖全部来自进程级的 ServiceContext
func NewLogic(ctx *svc.ServiceContext) *Logic {
	return &Logic{
		users:    userrepo.New(ctx.DB),
		follows:  followrepo.New(ctx.DB, ctx.Redis),
		videos:   videorepo.New(ctx.DB, ctx.Redis),
		auth:     ctx.Auth,
		uploader: ctx.Upload,
		producer: ctx.Producer,
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewLogic(ctx))
	group := r.Group("users")
	{
		group.POST("/login", controller.Login)
		group.POST("/register", controller.Register)
		group.POST("/refresh", controller.Refresh)
		group.DELETE("/logout", controller.Logout)
		group.GET("/me", ctx.Auth.Middleware(), controller.GetUserInfo)
		group.POST("/me", ctx.Auth.Middleware(), controller.UpdateProfile)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
	moduleLogic := NewLogic(ctx)
	sub.Subscribe(topic.FollowSwitched, moduleLogic.HandleFollowSwitched)
	sub.Subscribe(topic.VideoCreated, moduleLogic.HandleVideoCreated)
	sub.Subscribe(topic.VideoDeleted, moduleLogic.HandleVideoDeleted)
	return nil
}
