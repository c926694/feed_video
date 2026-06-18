package feed

import (
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, ctx.KafkaBrokers)
	httpHandler := NewHTTPHandler(feedService)
	feedGroup := r.Group("videos")
	{
		feedGroup.GET("/feed", middleware.JWTAuth(ctx.Redis), httpHandler.GetFeedVideos)
		feedGroup.GET("/feed/hot", middleware.JWTAuth(ctx.Redis), httpHandler.GetFeedHotVideos)
		feedGroup.GET("/feed/follow", middleware.JWTAuth(ctx.Redis), httpHandler.GetFollowFeedVideos)
	}
	return r, nil
}
