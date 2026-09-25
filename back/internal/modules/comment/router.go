package comment

import (
	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

func RegisterHTTP(r *gin.Engine, model *Model, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(model)
	group := r.Group("comments")
	{
		group.POST("", ctx.Auth.Middleware(), controller.Create)
		group.DELETE("/:id", ctx.Auth.Middleware(), controller.Delete)
		group.GET("/list/:videoId", ctx.Auth.Middleware(), controller.List)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, model *Model, ctx *svc.ServiceContext) error {
	sub.Subscribe(topic.LikeSwitched, model.HandleLikeSwitched)
	sub.Subscribe(topic.VideoDeleted, model.HandleVideoDeleted)
	sub.Subscribe(topic.UserUpdated, model.HandleUserUpdated)
	return nil
}
