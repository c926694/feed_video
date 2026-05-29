package svc

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ServiceContext struct {
	DB         *gorm.DB
	Redis      *redis.Client
	RabbitConn *amqp.Connection
}
