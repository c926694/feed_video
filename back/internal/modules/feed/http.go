package feed

import (
	"simple_tiktok/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	feedGroup := r.Group("videos")
	{
		feedGroup.GET("/feed", middleware.JWTAuth(m.redis), m.feedHandler.GetFeedVideos)
		feedGroup.GET("/feed/hot", middleware.JWTAuth(m.redis), m.feedHandler.GetFeedHotVideos)
		feedGroup.GET("/feed/follow", middleware.JWTAuth(m.redis), m.feedHandler.GetFollowFeedVideos)
	}
	return nil
}
