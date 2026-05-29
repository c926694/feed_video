package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"simple_tiktok/internal/app"
	initialize2 "simple_tiktok/internal/initialize"
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
	r, err := app.BuildHTTPFromContext(&svc.ServiceContext{
		DB:         initialize2.DB,
		Redis:      initialize2.RedisClient,
		RabbitConn: initialize2.RabbitConn,
	})
	if err != nil {
		log.Fatalf("init router failed: %v", err)
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
