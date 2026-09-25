package svc

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/kafka"
	"simple_tiktok/internal/platform/upload"
)

// ServiceContext 进程级的共享依赖，装配阶段逐个传给模块
type ServiceContext struct {
	DB        *gorm.DB
	Redis     *redis.Client
	Publisher *kafka.Publisher
	Auth      *auth.Service
	Upload    *upload.Uploader
}
