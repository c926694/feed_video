package comment

import (
	"simple_tiktok/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterHTTP(r *gin.Engine) (*gin.Engine, error) {
	commentGroup := r.Group("comments")
	{
		commentGroup.POST("", middleware.JWTAuth(m.redis), m.httpHandler.Create)
		commentGroup.DELETE("/:id", middleware.JWTAuth(m.redis), m.httpHandler.Delete)
		commentGroup.GET("/list/:videoId", middleware.JWTAuth(m.redis), m.httpHandler.List)
	}
	return r, nil
}
