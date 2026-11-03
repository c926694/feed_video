package message

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	commentrepo "simple_tiktok/internal/modules/comment/repo"
	messagerepo "simple_tiktok/internal/modules/message/repo"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

// Name 模块名，用作消费组的一部分，多个模块订阅同一个 topic 时互不争抢
const Name = "message"

// NewLogic 装配本模块的业务逻辑，依赖全部来自进程级的 ServiceContext
func NewLogic(ctx *svc.ServiceContext) *Logic {
	return &Logic{
		repo:     messagerepo.New(ctx.DB),
		users:    userrepo.New(ctx.DB),
		videos:   videorepo.New(ctx.DB, ctx.Redis),
		comments: commentrepo.New(ctx.DB),
		uploader: ctx.Upload,
	}
}

// NewMaintenanceLogic 供维护命令装配，只带补齐通知需要的仓储，不依赖上传配置
func NewMaintenanceLogic(db *gorm.DB, redisClient *redis.Client) *Logic {
	return &Logic{
		repo:     messagerepo.New(db),
		users:    userrepo.New(db),
		videos:   videorepo.New(db, redisClient),
		comments: commentrepo.New(db),
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewLogic(ctx))
	group := r.Group("messages")
	{
		group.GET("", ctx.Auth.Middleware(), controller.ListMine)
		group.GET("/unread", ctx.Auth.Middleware(), controller.Unread)
		group.POST("/read", ctx.Auth.Middleware(), controller.MarkRead)
	}
	return r, nil
}

// RegisterConsumers 订阅三类互动事件产生通知。收藏不通知
func RegisterConsumers(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
	moduleLogic := NewLogic(ctx)
	sub.Subscribe(topic.LikeSwitched, moduleLogic.HandleLikeSwitched)
	sub.Subscribe(topic.CommentCreated, moduleLogic.HandleCommentCreated)
	sub.Subscribe(topic.FollowSwitched, moduleLogic.HandleFollowSwitched)
	return nil
}
