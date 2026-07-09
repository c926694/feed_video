package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	initialize2 "simple_tiktok/internal/initialize"
	"simple_tiktok/internal/modules/comment"
	"simple_tiktok/internal/modules/feed"
	"simple_tiktok/internal/modules/follow"
	"simple_tiktok/internal/modules/like"
	"simple_tiktok/internal/modules/user"
	"simple_tiktok/internal/modules/video"
	"simple_tiktok/internal/svc"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := initialize2.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	if _, err = initialize2.InitMySQL(cfg.MySQL); err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	defer initialize2.CloseMySQL()

	if _, err = initialize2.InitRedis(cfg.Redis); err != nil {
		log.Fatalf("init redis failed: %v", err)
	}
	defer initialize2.CloseRedis()

	if _, err = initialize2.InitKafka(cfg.Kafka); err != nil {
		log.Fatalf("init kafka failed: %v", err)
	}
	defer initialize2.CloseKafka()

	if err = initialize2.AutoMigrate(initialize2.DB); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}

	if err = ensureUploadDirs(cfg.Upload.BasePath, cfg.Upload.AvatarDir, cfg.Upload.CoverDir, cfg.Upload.VideoDir); err != nil {
		log.Fatalf("create upload directories failed: %v", err)
	}

	gin.SetMode(cfg.Server.Mode)
	ctx := &svc.ServiceContext{
		DB:           initialize2.DB,
		Redis:        initialize2.RedisClient,
		KafkaBrokers: cfg.Kafka.Brokers,
	}
	r := gin.Default()
	if _, err := user.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("register user routes failed: %v", err)
	}
	if _, err := video.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("register video routes failed: %v", err)
	}
	if _, err := comment.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("register comment routes failed: %v", err)
	}
	if _, err := like.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("register like routes failed: %v", err)
	}
	if _, err := follow.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("register follow routes failed: %v", err)
	}
	if _, err := feed.RegisterHTTP(r, ctx); err != nil {
		log.Fatalf("register feed routes failed: %v", err)
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Println("监听端口:", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		<-runCtx.Done()
		log.Println("收到停止信号，正在关闭 http 服务")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Printf("shutdown http failed: %v", shutdownErr)
		}
	}()

	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("start http failed: %v", err)
	}
}

func ensureUploadDirs(basePath string, dirs ...string) error {
	for _, dir := range dirs {
		target := filepath.Join(basePath, dir)
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
	}
	return nil
}
