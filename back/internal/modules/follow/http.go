package follow

import (
	"simple_tiktok/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterHTTP(r *gin.Engine) (*gin.Engine, error) {
	followGroup := r.Group("follows")
	{
		followGroup.POST("/switchFollow/:follower", middleware.JWTAuth(m.redis), m.httpHandler.Follow)
	}
	return r, nil
}
