package video

import (
	"context"

	"github.com/gin-gonic/gin"

	followrepo "simple_tiktok/internal/modules/follow/repo"
	likerepo "simple_tiktok/internal/modules/like/repo"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

// Name 模块名，用作消费组的一部分，多个模块订阅同一个 topic 时互不争抢
const Name = "video"

// NewModel 装配本模块的业务层。自己的数据走 videorepo，
// 别人的数据走别人的 repo，依赖全部来自进程级的 ServiceContext
func NewModel(ctx *svc.ServiceContext) *Model {
	return &Model{
		videos:   videorepo.New(ctx.DB, ctx.Redis),
		users:    userrepo.New(ctx.DB),
		likes:    likerepo.New(ctx.Redis),
		follows:  followrepo.New(ctx.DB, ctx.Redis),
		producer: ctx.Producer,
		uploader: ctx.Upload,
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewModel(ctx))
	group := r.Group("videos")
	{
		group.POST("/create", ctx.Auth.Middleware(), controller.CreateVideo)
		group.DELETE("/:id", ctx.Auth.Middleware(), controller.DeleteVideos)
		group.GET("/me", ctx.Auth.Middleware(), controller.GetMyVideos)
		group.GET("/:id", ctx.Auth.Middleware(), controller.GetVideoInfo)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
	model := NewModel(ctx)
	// 删除视频时清理 OSS 上的文件
	sub.Subscribe(topic.VideoDeleted, func(handlerCtx context.Context, payload []byte) error {
		return handleVideoDeleted(handlerCtx, payload, ctx.Upload)
	})
	sub.Subscribe(topic.LikeSwitched, model.HandleLikeSwitched)
	sub.Subscribe(topic.CommentCreated, model.HandleCommentCreated)
	sub.Subscribe(topic.CommentDeleted, model.HandleCommentDeleted)
	sub.Subscribe(topic.UserUpdated, model.HandleUserUpdated)
	return nil
}
