package follow

import (
	"github.com/gin-gonic/gin"

	followrepo "simple_tiktok/internal/modules/follow/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

// Name 模块名，用作消费组的一部分，多个模块订阅同一个 topic 时互不争抢
const Name = "follow"

// NewModel 装配本模块的业务层，依赖全部来自进程级的 ServiceContext
func NewModel(ctx *svc.ServiceContext) *Model {
	return &Model{
		repo:     followrepo.New(ctx.DB, ctx.Redis),
		producer: ctx.Producer,
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewModel(ctx))
	group := r.Group("follows")
	{
		group.POST("/switchFollow/:follower", ctx.Auth.Middleware(), controller.Follow)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
	sub.Subscribe(topic.FollowSwitched, NewModel(ctx).HandleSwitched)
	return nil
}
