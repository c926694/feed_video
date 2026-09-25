package svc

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/upload"
)

// ServiceContext 进程级的中间件与共享依赖。模块实例在启动代码里显式传递，
// 不放进这里，避免 svc 与模块之间形成 import 环。
type ServiceContext struct {
	DB       *gorm.DB
	Redis    *redis.Client
	Producer *producer.Producer
	Auth     *auth.Service
	Upload   *upload.Uploader
}
