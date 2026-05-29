package comment

import (
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/svc"
	"simple_tiktok/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	commentRepo := NewCommentRepo(ctx.DB)
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	hotMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, hotMQ)
	commentService := NewService(commentRepo, videoRepo, userRepo, ctx.Redis, feedService)
	httpHandler := NewHTTPHandler(commentService)
	commentGroup := r.Group("comments")
	{
		commentGroup.POST("", middleware.JWTAuth(ctx.Redis), httpHandler.Create)
		commentGroup.DELETE("/:id", middleware.JWTAuth(ctx.Redis), httpHandler.Delete)
		commentGroup.GET("/list/:videoId", middleware.JWTAuth(ctx.Redis), httpHandler.List)
	}
	return r, nil
}
