package feed

import (
	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/svc"
)

func RegisterHTTP(r *gin.Engine, model *Model, ctx *svc.ServiceContext) (*gin.Engine, error) {
	controller := NewController(model)
	group := r.Group("videos")
	{
		group.GET("/feed", ctx.Auth.Middleware(), controller.GetFeedVideos)
		group.GET("/feed/hot", ctx.Auth.Middleware(), controller.GetFeedHotVideos)
		group.GET("/feed/follow", ctx.Auth.Middleware(), controller.GetFollowFeedVideos)
	}
	return r, nil
}

func RegisterConsumers(sub *consumer.Consumer, model *Model, ctx *svc.ServiceContext) error {
	sub.Subscribe(topic.VideoCreated, model.HandleVideoCreated)
	sub.Subscribe(topic.VideoDeleted, model.HandleVideoDeleted)
	sub.Subscribe(topic.LikeSwitched, model.HandleLikeSwitched)
	sub.Subscribe(topic.CommentCreated, model.HandleCommentCreated)
	sub.Subscribe(topic.CommentDeleted, model.HandleCommentDeleted)
	return nil
}
