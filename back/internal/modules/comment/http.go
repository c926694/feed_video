package comment

import (
	"simple_tiktok/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	commentGroup := r.Group("comments")
	{
		commentGroup.POST("", middleware.JWTAuth(m.redis), m.controller.Create)
		commentGroup.DELETE("/:id", middleware.JWTAuth(m.redis), m.controller.Delete)
		commentGroup.GET("/list/:videoId", middleware.JWTAuth(m.redis), m.controller.List)
	}
	return nil
}
