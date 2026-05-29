package user

import (
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func RegisterHTTP(r *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
	userRepo := NewUserRepo(ctx.DB)
	videoRepo := NewVideoRepo(ctx.DB)
	userService := NewService(userRepo, videoRepo, ctx.Redis)
	httpHandler := NewHTTPHandler(userService)
	userGroup := r.Group("users")
	{
		userGroup.POST("/login", httpHandler.Login)
		userGroup.POST("/register", httpHandler.Register)
		userGroup.DELETE("/logout", middleware.JWTAuth(ctx.Redis), httpHandler.Logout)
		userGroup.GET("/me", middleware.JWTAuth(ctx.Redis), httpHandler.GetUserInfo)
		userGroup.POST("/me", middleware.JWTAuth(ctx.Redis), httpHandler.UpdateProfile)
	}
	return r, nil
}
