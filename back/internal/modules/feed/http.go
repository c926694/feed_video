package feed

import (
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, ctx.Publisher, ctx.Upload)
	httpHandler := NewHTTPHandler(feedService)
	feedGroup := r.Group("videos")
	{
		feedGroup.GET("/feed", ctx.Auth.Middleware(), httpHandler.GetFeedVideos)
		feedGroup.GET("/feed/hot", ctx.Auth.Middleware(), httpHandler.GetFeedHotVideos)
		feedGroup.GET("/feed/follow", ctx.Auth.Middleware(), httpHandler.GetFollowFeedVideos)
	}
	return r, nil
}
