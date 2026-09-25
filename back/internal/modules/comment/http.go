package comment

import (
	"simple_tiktok/internal/service"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	commentRepo := NewCommentRepo(ctx.DB)
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, ctx.Publisher, ctx.Upload)
	commentService := NewService(commentRepo, videoRepo, userRepo, ctx.Redis, feedService, ctx.Upload)
	httpHandler := NewHTTPHandler(commentService)
	commentGroup := r.Group("comments")
	{
		commentGroup.POST("", ctx.Auth.Middleware(), httpHandler.Create)
		commentGroup.DELETE("/:id", ctx.Auth.Middleware(), httpHandler.Delete)
		commentGroup.GET("/list/:videoId", ctx.Auth.Middleware(), httpHandler.List)
	}
	return r, nil
}
