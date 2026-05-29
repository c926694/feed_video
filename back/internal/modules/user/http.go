package user

import (
	"simple_tiktok/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterHTTP(r *gin.Engine) (*gin.Engine, error) {
	userGroup := r.Group("users")
	{
		userGroup.POST("/login", m.httpHandler.Login)
		userGroup.POST("/register", m.httpHandler.Register)
		userGroup.DELETE("/logout", middleware.JWTAuth(m.redis), m.httpHandler.Logout)
		userGroup.GET("/me", middleware.JWTAuth(m.redis), m.httpHandler.GetUserInfo)
		userGroup.POST("/me", middleware.JWTAuth(m.redis), m.httpHandler.UpdateProfile)
	}
	return r, nil
}
