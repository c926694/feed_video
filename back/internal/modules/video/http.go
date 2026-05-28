package video

import (
	"simple_tiktok/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	videoGroup := r.Group("videos")
	{
		videoGroup.POST("/create", middleware.JWTAuth(m.redis), m.httpHandler.CreateVideo)
		videoGroup.DELETE("/:id", middleware.JWTAuth(m.redis), m.httpHandler.DeleteVideos)
		videoGroup.GET("/me", middleware.JWTAuth(m.redis), m.httpHandler.GetMyVideos)
		videoGroup.GET("/:id", middleware.JWTAuth(m.redis), m.httpHandler.GetVideoInfo)
	}
	return nil
}
