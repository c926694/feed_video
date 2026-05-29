package router

import (
	"simple_tiktok/internal/modules/comment"
	"simple_tiktok/internal/modules/feed"
	"simple_tiktok/internal/modules/follow"
	"simple_tiktok/internal/modules/like"
	"simple_tiktok/internal/modules/user"
	"simple_tiktok/internal/modules/video"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB, redisClient *redis.Client, conn *amqp.Connection) (*gin.Engine, error) {
	ctx := &svc.ServiceContext{
		DB:         db,
		Redis:      redisClient,
		RabbitConn: conn,
	}
	r := gin.Default()
	if _, err := user.RegisterHTTP(r, ctx); err != nil {
		return nil, err
	}
	if _, err := video.RegisterHTTP(r, ctx); err != nil {
		return nil, err
	}
	if _, err := comment.RegisterHTTP(r, ctx); err != nil {
		return nil, err
	}
	if _, err := like.RegisterHTTP(r, ctx); err != nil {
		return nil, err
	}
	if _, err := follow.RegisterHTTP(r, ctx); err != nil {
		return nil, err
	}
	if _, err := feed.RegisterHTTP(r, ctx); err != nil {
		return nil, err
	}
	return r, nil
}
