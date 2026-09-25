package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"simple_tiktok/internal/modules/feed"
	"simple_tiktok/internal/modules/follow"
	"simple_tiktok/internal/modules/like"
	"simple_tiktok/internal/modules/video"
	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/config"
	"simple_tiktok/internal/platform/kafka"
	"simple_tiktok/internal/platform/mysql"
	"simple_tiktok/internal/platform/redis"
	"simple_tiktok/internal/platform/upload"
	"simple_tiktok/internal/svc"
)

func main() {
	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	db, err := mysql.Connect(cfg.MySQL)
	if err != nil {
		log.Fatalf("连接 mysql 失败: %v", err)
	}
	defer func() {
		if closeErr := mysql.Close(db); closeErr != nil {
			log.Printf("关闭 mysql 失败: %v", closeErr)
		}
	}()

	redisClient, err := redis.Connect(cfg.Redis)
	if err != nil {
		log.Fatalf("连接 redis 失败: %v", err)
	}
	defer func() {
		if closeErr := redis.Close(redisClient); closeErr != nil {
			log.Printf("关闭 redis 失败: %v", closeErr)
		}
	}()

	publisher, err := kafka.NewPublisher(cfg.Kafka.Brokers)
	if err != nil {
		log.Fatalf("初始化 kafka 失败: %v", err)
	}
	publisher.EnsureTopics(runCtx)
	defer func() {
		if closeErr := publisher.Close(); closeErr != nil {
			log.Printf("关闭 kafka 生产者失败: %v", closeErr)
		}
	}()

	authService, err := auth.New(cfg.JWT.Secret, cfg.JWT.ExpireHours, redisClient)
	if err != nil {
		log.Fatalf("初始化鉴权失败: %v", err)
	}

	uploader, err := upload.New(cfg.Upload)
	if err != nil {
		log.Fatalf("初始化上传目录失败: %v", err)
	}

	ctx := &svc.ServiceContext{
		DB:        db,
		Redis:     redisClient,
		Publisher: publisher,
		Auth:      authService,
		Upload:    uploader,
	}
	subscriber := kafka.NewSubscriber(publisher, cfg.Kafka.GroupID)

	if err = like.RegisterConsumers(subscriber, ctx); err != nil {
		log.Fatalf("注册 like 消费者失败: %v", err)
	}
	if err = follow.RegisterConsumers(subscriber, ctx); err != nil {
		log.Fatalf("注册 follow 消费者失败: %v", err)
	}
	if err = video.RegisterConsumers(subscriber, ctx); err != nil {
		log.Fatalf("注册 video 消费者失败: %v", err)
	}
	if err = feed.RegisterConsumers(subscriber, ctx); err != nil {
		log.Fatalf("注册 feed 消费者失败: %v", err)
	}

	log.Println("开始监听 kafka")
	if err = subscriber.Run(runCtx); err != nil {
		log.Fatalf("消费失败: %v", err)
	}
}
