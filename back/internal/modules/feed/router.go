package feed

import (
	"github.com/gin-gonic/gin"

	feedrepo "simple_tiktok/internal/modules/feed/repo"
	followrepo "simple_tiktok/internal/modules/follow/repo"
	likerepo "simple_tiktok/internal/modules/like/repo"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

// Name 模块名，用作消费组的一部分，多个模块订阅同一个 topic 时互不争抢
const Name = "feed"

// NewModel 装配本模块的业务层。索引与热度走 feedrepo，
// 视频与用户数据走别人的 repo，依赖全部来自进程级的 ServiceContext
func NewModel(ctx *svc.ServiceContext) *Model {
	return &Model{
		feed:     feedrepo.New(ctx.Redis),
		videos:   videorepo.New(ctx.DB, ctx.Redis),
		users:    userrepo.New(ctx.DB),
		likes:    likerepo.New(ctx.Redis),
		follows:  followrepo.New(ctx.DB, ctx.Redis),
		uploader: ctx.Upload,
	}
}

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(NewModel(ctx))
	group := r.Group("videos")
	{
		group.GET("/feed", ctx.Auth.Middleware(), controller.GetFeedVideos)
		group.GET("/feed/hot", ctx.Auth.Middleware(), controller.GetFeedHotVideos)
		group.GET("/feed/follow", ctx.Auth.Middleware(), controller.GetFollowFeedVideos)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
	model := NewModel(ctx)
	sub.Subscribe(topic.VideoCreated, model.HandleVideoCreated)
	sub.Subscribe(topic.VideoDeleted, model.HandleVideoDeleted)
	sub.Subscribe(topic.LikeSwitched, model.HandleLikeSwitched)
	sub.Subscribe(topic.CommentCreated, model.HandleCommentCreated)
	sub.Subscribe(topic.CommentDeleted, model.HandleCommentDeleted)
	return nil
}
