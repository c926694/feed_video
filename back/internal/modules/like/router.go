package like

import (
	"github.com/gin-gonic/gin"

	commentrepo "simple_tiktok/internal/modules/comment/repo"
	"simple_tiktok/internal/modules/like/repo"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

// Name 模块名，用作消费组的一部分，多个模块订阅同一个 topic 时互不争抢
const Name = "like"

// NewLogic 装配本模块的业务逻辑，依赖全部来自进程级的 ServiceContext
func NewLogic(ctx *svc.ServiceContext) *Logic {
	return &Logic{
		repo:     repo.New(ctx.DB, ctx.Redis),
		videos:   videorepo.New(ctx.DB, ctx.Redis),
		comments: commentrepo.New(ctx.DB),
		producer: ctx.Producer,
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewLogic(ctx))
	group := r.Group("likes")
	{
		group.POST("/video/switchLike/:id", ctx.Auth.Middleware(), controller.LikeVideo)
		group.POST("/comment/switchLike/:id", ctx.Auth.Middleware(), controller.LikeComment)
	}
	return r, nil
}

// RegisterConsumers 订阅点赞事件维护关系表，以及删除事件清理点赞
func RegisterConsumers(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
	moduleLogic := NewLogic(ctx)
	sub.Subscribe(topic.LikeSwitched, moduleLogic.HandleSwitched)
	sub.Subscribe(topic.VideoDeleted, moduleLogic.HandleVideoDeleted)
	sub.Subscribe(topic.CommentDeleted, moduleLogic.HandleCommentDeleted)
	return nil
}
