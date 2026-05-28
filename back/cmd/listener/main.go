package main

import (
	"log"
	initialize2 "simple_tiktok/internal/initialize"
	consumer2 "simple_tiktok/internal/mq/consumer"
	"simple_tiktok/internal/mq/event"
	mysql2 "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"
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
	videoRepo := mysql2.NewVideoRepo(initialize2.DB)
	commentRepo := mysql2.NewCommentRepo(initialize2.DB)
	followRepo := mysql2.NewFollowRepo(initialize2.DB)
	userRepo := mysql2.NewUserRepo(initialize2.DB)
	videoRedis := initialize2.RedisClient
	videoService := service.NewVideoService(videoRepo, userRepo, videoRedis, initialize2.RabbitChannel, commentRepo)
	likeConsumer := consumer2.NewLikeConsumer(initialize2.RabbitChannel, videoRepo, commentRepo, initialize2.RedisClient)
	videoHotConsumer := consumer2.NewVideoHotConsumer(initialize2.RabbitChannel, videoRepo, videoService)
	followConsumer := consumer2.NewFollowConsumer(initialize2.RabbitChannel, followRepo, userRepo)
	err = likeConsumer.Declare(event.LikeVideoExchange, event.LikeVideoExchangeType,
		event.LikeVideoQueue, event.LikeVideoRoutingKey)
	if err != nil {
		log.Fatalf("likeConsumer declare error: %s", err.Error())
	}
	err = likeConsumer.Declare(event.LikeCommentExchange, event.LikeCommentExchangeType,
		event.LikeCommentQueue, event.LikeCommentRoutingKey)
	if err != nil {
		log.Fatalf("likeCommentConsumer declare error: %s", err.Error())
	}
	err = followConsumer.Declare(event.FollowExchange, event.FollowExchangeType,
		event.FollowQueue, event.FollowRoutingKey)
	if err != nil {
		log.Fatalf("followConsumer declare error: %s", err.Error())
	}
	err = videoHotConsumer.Declare(event.VideoHotExchange, event.VideoHotExchangeType,
		event.VideoHotQueue, event.VideoHotRoutingKey)
	if err != nil {
		log.Fatalf("videoHotConsumer declare error: %s", err.Error())
	}
	videoConsumer := consumer2.NewVideoConsumer(initialize2.RabbitChannel, videoRepo)
	err = videoConsumer.Declare(event.DeleteVideoExchange, event.DeleteVideoExchangeType,
		event.DeleteVideoQueue, event.DeleteVideoRoutingKey)
	if err != nil {
		log.Fatalf("videoConsumer declare error: %s", err.Error())
	}
	errCh := make(chan error, 5)
	go func() {
		errCh <- likeConsumer.ListenLikeConsumer(event.LikeVideoQueue, likeConsumer.LikeVideoHandler)
	}()
	go func() {
		errCh <- likeConsumer.ListenLikeConsumer(event.LikeCommentQueue, likeConsumer.LikeCommentHandler)
	}()
	go func() {
		errCh <- followConsumer.ListenFollowConsumer(event.FollowQueue, followConsumer.FollowHandler)
	}()
	go func() {
		errCh <- videoHotConsumer.Listen(event.VideoHotQueue, videoHotConsumer.HotUpdateHandler)
	}()
	log.Println("开始监听mq")
	go func() {
		errCh <- videoConsumer.ListenVideoConsumer(event.DeleteVideoQueue, videoConsumer.DeleteVideoHandler)
	}()
	err = <-errCh
	if err != nil {
		log.Fatalf("likeConsumer Listen error: %s", err.Error())
	}
}
