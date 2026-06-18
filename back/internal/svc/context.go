package svc

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ServiceContext struct {
	DB           *gorm.DB
	Redis        *redis.Client
	KafkaBrokers []string
}
