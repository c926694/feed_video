package user

import (
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	userRepo := NewUserRepo(ctx.DB)
	videoRepo := NewVideoRepo(ctx.DB)
	userService := NewService(userRepo, videoRepo, ctx.Auth, ctx.Upload)
	httpHandler := NewHTTPHandler(userService)
	userGroup := r.Group("users")
	{
		userGroup.POST("/login", httpHandler.Login)
		userGroup.POST("/register", httpHandler.Register)
		userGroup.DELETE("/logout", ctx.Auth.Middleware(), httpHandler.Logout)
		userGroup.GET("/me", ctx.Auth.Middleware(), httpHandler.GetUserInfo)
		userGroup.POST("/me", ctx.Auth.Middleware(), httpHandler.UpdateProfile)
	}
	return r, nil
}
