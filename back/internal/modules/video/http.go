package video

import (
	"simple_tiktok/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	videoGroup := r.Group("videos")
	{
		videoGroup.POST("/create", middleware.JWTAuth(m.redis), m.controller.CreateVideo)
		videoGroup.DELETE("/:id", middleware.JWTAuth(m.redis), m.controller.DeleteVideos)
		videoGroup.GET("/me", middleware.JWTAuth(m.redis), m.controller.GetMyVideos)
		videoGroup.GET("/:id", middleware.JWTAuth(m.redis), m.controller.GetVideoInfo)
	}
	return nil
}
