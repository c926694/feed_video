package like

import (
	"simple_tiktok/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterHTTP(r *gin.Engine) (*gin.Engine, error) {
	likeGroup := r.Group("likes")
	{
		likeGroup.POST("/video/switchLike/:id", middleware.JWTAuth(m.redis), m.httpHandler.LikeVideo)
		likeGroup.POST("/comment/switchLike/:id", middleware.JWTAuth(m.redis), m.httpHandler.LikeComment)
	}
	return r, nil
}
