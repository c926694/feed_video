package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	initialize2 "simple_tiktok/internal/initialize"
	"simple_tiktok/internal/modules/comment"
	"simple_tiktok/internal/modules/feed"
	"simple_tiktok/internal/modules/follow"
	"simple_tiktok/internal/modules/like"
	"simple_tiktok/internal/modules/user"
	"simple_tiktok/internal/modules/video"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := initialize2.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	if _, err = initialize2.InitMySQL(cfg.MySQL); err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}

	if _, err = initialize2.InitRedis(cfg.Redis); err != nil {
		log.Fatalf("init redis failed: %v", err)
	}

	if _, _, err = initialize2.InitRabbitMQ(cfg.RabbitMQ); err != nil {
		log.Fatalf("init rabbitmq failed: %v", err)
	}
	defer initialize2.CloseRabbitMQ()

	if err = initialize2.AutoMigrate(initialize2.DB); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}

	if err = ensureUploadDirs(cfg.Upload.BasePath, cfg.Upload.AvatarDir, cfg.Upload.CoverDir, cfg.Upload.VideoDir); err != nil {
		log.Fatalf("create upload directories failed: %v", err)
	}

	gin.SetMode(cfg.Server.Mode)
	ctx := &svc.ServiceContext{
		DB:         initialize2.DB,
		Redis:      initialize2.RedisClient,
		RabbitConn: initialize2.RabbitConn,
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
	if err = r.Run(addr); err != nil {
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
