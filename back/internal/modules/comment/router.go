package comment

import (
	"github.com/gin-gonic/gin"

	commentrepo "simple_tiktok/internal/modules/comment/repo"
	likerepo "simple_tiktok/internal/modules/like/repo"
	userrepo "simple_tiktok/internal/modules/user/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

// Name 模块名，用作消费组的一部分，多个模块订阅同一个 topic 时互不争抢
const Name = "comment"

// NewLogic 装配本模块的业务层。自己的数据走 commentrepo，
// 别人的数据走别人的 repo，依赖全部来自进程级的 ServiceContext
func NewLogic(ctx *svc.ServiceContext) *Logic {
	return &Logic{
		comments: commentrepo.New(ctx.DB),
		users:    userrepo.New(ctx.DB),
		likes:    likerepo.New(ctx.Redis),
		producer: ctx.Producer,
		uploader: ctx.Upload,
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewLogic(ctx))
	group := r.Group("comments")
	{
		group.POST("", ctx.Auth.Middleware(), controller.Create)
		group.DELETE("/:id", ctx.Auth.Middleware(), controller.Delete)
		group.GET("/list/:videoId", ctx.Auth.Middleware(), controller.List)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
	moduleLogic := NewLogic(ctx)
	sub.Subscribe(topic.LikeSwitched, moduleLogic.HandleLikeSwitched)
	sub.Subscribe(topic.VideoDeleted, moduleLogic.HandleVideoDeleted)
	sub.Subscribe(topic.UserUpdated, moduleLogic.HandleUserUpdated)
	return nil
}
