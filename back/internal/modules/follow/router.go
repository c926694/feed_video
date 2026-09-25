package follow

import (
	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

func RegisterHTTP(r *gin.Engine, model *Model, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(model)
	group := r.Group("follows")
	{
		group.POST("/switchFollow/:follower", ctx.Auth.Middleware(), controller.Follow)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, model *Model, ctx *svc.ServiceContext) error {
	sub.Subscribe(topic.FollowSwitched, model.HandleSwitched)
	return nil
}
