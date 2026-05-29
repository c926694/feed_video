package router

import (
	"simple_tiktok/internal/app"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB, redisClient *redis.Client, conn *amqp.Connection) (*gin.Engine, error) {
	return app.BuildHTTPFromContext(&svc.ServiceContext{
		DB:         db,
		Redis:      redisClient,
		RabbitConn: conn,
	})
}
