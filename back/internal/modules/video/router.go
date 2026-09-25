package video

import (
	"context"

	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

func RegisterHTTP(r *gin.Engine, model *Model, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(model)
	group := r.Group("videos")
	{
		group.POST("/create", ctx.Auth.Middleware(), controller.CreateVideo)
		group.DELETE("/:id", ctx.Auth.Middleware(), controller.DeleteVideos)
		group.GET("/me", ctx.Auth.Middleware(), controller.GetMyVideos)
		group.GET("/:id", ctx.Auth.Middleware(), controller.GetVideoInfo)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, model *Model, ctx *svc.ServiceContext) error {
	// 删除视频时清理存储目录里的文件
	sub.Subscribe(topic.VideoDeleted, func(handlerCtx context.Context, payload []byte) error {
		return handleVideoDeleted(handlerCtx, payload, ctx.Upload)
	})
	sub.Subscribe(topic.LikeSwitched, model.HandleLikeSwitched)
	sub.Subscribe(topic.CommentCreated, model.HandleCommentCreated)
	sub.Subscribe(topic.CommentDeleted, model.HandleCommentDeleted)
	sub.Subscribe(topic.UserUpdated, model.HandleUserUpdated)
	return nil
}
