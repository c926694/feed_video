package user

import (
	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

func RegisterHTTP(r *gin.Engine, model *Model, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(model)
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

func RegisterConsumers(sub *consumer.Consumer, model *Model, ctx *svc.ServiceContext) error {
	sub.Subscribe(topic.FollowSwitched, model.HandleFollowSwitched)
	sub.Subscribe(topic.VideoCreated, model.HandleVideoCreated)
	sub.Subscribe(topic.VideoDeleted, model.HandleVideoDeleted)
	return nil
}
