package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"simple_tiktok/internal/platform/config"
)

const pingTimeout = 3 * time.Second

// Connect 建立 Redis 连接并确认可用
func Connect(cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接 redis 失败: %w", err)
	}
	return client, nil
}

// Close 关闭连接
func Close(client *redis.Client) error {
	if client == nil {
		return nil
	}
	return client.Close()
}
