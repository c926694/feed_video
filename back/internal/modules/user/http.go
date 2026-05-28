package user

import (
	"simple_tiktok/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	userGroup := r.Group("users")
	{
		userGroup.POST("/login", m.controller.Login)
		userGroup.POST("/register", m.controller.Register)
		userGroup.DELETE("/logout", middleware.JWTAuth(m.redis), m.controller.Logout)
		userGroup.GET("/me", middleware.JWTAuth(m.redis), m.controller.GetUserInfo)
		userGroup.POST("/me", middleware.JWTAuth(m.redis), m.controller.UpdateProfile)
	}
	return nil
}
