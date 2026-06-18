package modulekit

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Context struct {
	DB           *gorm.DB
	Redis        *redis.Client
	KafkaBrokers []string
}
