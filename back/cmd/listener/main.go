package main

import (
	"log"
	"simple_tiktok/internal/app"
	initialize2 "simple_tiktok/internal/initialize"
	"simple_tiktok/internal/modules/feed"
	"simple_tiktok/internal/modules/follow"
	"simple_tiktok/internal/modules/like"
	"simple_tiktok/internal/modules/video"
	"simple_tiktok/internal/svc"
)

func main() {
	cfg, err := initialize2.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("init config err: %v", err)
	}
	if _, err := initialize2.InitMySQL(cfg.MySQL); err != nil {
		log.Fatalf("init mysql err: %v", err)
	}
	if _, err := initialize2.InitRedis(cfg.Redis); err != nil {
		log.Fatalf("init redis err: %v", err)
	}
	if _, _, err := initialize2.InitRabbitMQ(cfg.RabbitMQ); err != nil {
		log.Fatalf("init rabbitmq err: %v", err)
	}
	ctx := &svc.ServiceContext{
		DB:         initialize2.DB,
		Redis:      initialize2.RedisClient,
		RabbitConn: initialize2.RabbitConn,
	}
	runner := app.NewConsumerRunner()
	if err := like.RegisterConsumers(runner, ctx); err != nil {
		log.Fatalf("register like consumers err: %v", err)
	}
	if err := follow.RegisterConsumers(runner, ctx); err != nil {
		log.Fatalf("register follow consumers err: %v", err)
	}
	if err := video.RegisterConsumers(runner, ctx); err != nil {
		log.Fatalf("register video consumers err: %v", err)
	}
	if err := feed.RegisterConsumers(runner, ctx); err != nil {
		log.Fatalf("register feed consumers err: %v", err)
	}
	log.Println("开始监听mq")
	errCh := runner.StartAll()
	err = <-errCh
	if err != nil {
		log.Fatalf("consumer listen error: %s", err.Error())
	}
}
