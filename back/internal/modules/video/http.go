package video

import (
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	commentRepo := NewCommentRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, ctx.Publisher, ctx.Upload)
	videoService := NewService(videoRepo, userRepo, ctx.Redis, ctx.Publisher, commentRepo, feedService, ctx.Upload)
	httpHandler := NewHTTPHandler(videoService)
	videoGroup := r.Group("videos")
	{
		videoGroup.POST("/create", ctx.Auth.Middleware(), httpHandler.CreateVideo)
		videoGroup.DELETE("/:id", ctx.Auth.Middleware(), httpHandler.DeleteVideos)
		videoGroup.GET("/me", ctx.Auth.Middleware(), httpHandler.GetMyVideos)
		videoGroup.GET("/:id", ctx.Auth.Middleware(), httpHandler.GetVideoInfo)
	}
	return r, nil
}
