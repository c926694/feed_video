package video

import (
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	commentRepo := NewCommentRepo(ctx.DB)
	hotMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, hotMQ)

	videoMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoService := NewService(videoRepo, userRepo, ctx.Redis, videoMQ, commentRepo, feedService)
	httpHandler := NewHTTPHandler(videoService)
	videoGroup := r.Group("videos")
	{
		videoGroup.POST("/create", middleware.JWTAuth(ctx.Redis), httpHandler.CreateVideo)
		videoGroup.DELETE("/:id", middleware.JWTAuth(ctx.Redis), httpHandler.DeleteVideos)
		videoGroup.GET("/me", middleware.JWTAuth(ctx.Redis), httpHandler.GetMyVideos)
		videoGroup.GET("/:id", middleware.JWTAuth(ctx.Redis), httpHandler.GetVideoInfo)
	}
	return r, nil
}
