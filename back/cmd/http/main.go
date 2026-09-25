package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/model"
	"simple_tiktok/internal/modules/comment"
	"simple_tiktok/internal/modules/feed"
	"simple_tiktok/internal/modules/follow"
	"simple_tiktok/internal/modules/like"
	"simple_tiktok/internal/modules/user"
	"simple_tiktok/internal/modules/video"
	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/config"
	"simple_tiktok/internal/platform/httpx"
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

	if err = mysql.Migrate(db,
		&model.User{},
		&model.Video{},
		&model.Comment{},
		&model.Follow{},
	); err != nil {
		log.Fatalf("建立表结构失败: %v", err)
	}

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

	gin.SetMode(cfg.Server.Mode)
	ctx := &svc.ServiceContext{
		DB:        db,
		Redis:     redisClient,
		Publisher: publisher,
		Auth:      authService,
		Upload:    uploader,
	}

	r := gin.Default()
	r.HandleMethodNotAllowed = true
	r.NoRoute(func(c *gin.Context) {
		httpx.Fail(c, httpx.New(httpx.CodeNotFound, "接口不存在"))
	})
	r.NoMethod(func(c *gin.Context) {
		httpx.Fail(c, httpx.New(httpx.CodeMethodNotAllowed, "请求方法不允许"))
	})
	if _, err = user.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("注册 user 路由失败: %v", err)
	}
	if _, err = video.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("注册 video 路由失败: %v", err)
	}
	if _, err = comment.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("注册 comment 路由失败: %v", err)
	}
	if _, err = like.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("注册 like 路由失败: %v", err)
	}
	if _, err = follow.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("注册 follow 路由失败: %v", err)
	}
	if _, err = feed.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("注册 feed 路由失败: %v", err)
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Println("监听端口:", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	go func() {
		<-runCtx.Done()
		log.Println("收到停止信号，正在关闭 http 服务")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Printf("关闭 http 服务失败: %v", shutdownErr)
		}
	}()

	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("启动 http 服务失败: %v", err)
	}
}
