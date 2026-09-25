package user

import (
	"github.com/gin-gonic/gin"

	userrepo "simple_tiktok/internal/modules/user/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

// Name 模块名，用作消费组的一部分，多个模块订阅同一个 topic 时互不争抢
const Name = "user"

// NewModel 装配本模块的业务层，依赖全部来自进程级的 ServiceContext
func NewModel(ctx *svc.ServiceContext) *Model {
	return &Model{
		users:    userrepo.New(ctx.DB),
		auth:     ctx.Auth,
		uploader: ctx.Upload,
		producer: ctx.Producer,
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewModel(ctx))
	group := r.Group("users")
	{
		group.POST("/login", controller.Login)
		group.POST("/register", controller.Register)
		group.DELETE("/logout", ctx.Auth.Middleware(), controller.Logout)
		group.GET("/me", ctx.Auth.Middleware(), controller.GetUserInfo)
		group.POST("/me", ctx.Auth.Middleware(), controller.UpdateProfile)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
	model := NewModel(ctx)
	sub.Subscribe(topic.FollowSwitched, model.HandleFollowSwitched)
	sub.Subscribe(topic.VideoCreated, model.HandleVideoCreated)
	sub.Subscribe(topic.VideoDeleted, model.HandleVideoDeleted)
	return nil
}
