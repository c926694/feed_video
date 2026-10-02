package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	followrepo "simple_tiktok/internal/modules/follow/repo"
	"simple_tiktok/internal/platform/config"
	"simple_tiktok/internal/platform/mysql"
	"simple_tiktok/internal/platform/redis"
)

// 从关注表重建两个方向的集合。Redis 数据丢失后靠它恢复，
// 关注集合本身只有关注接口一个写入方
func main() {
	path, err := resolveConfigPath()
	if err != nil {
		log.Fatalf("定位配置文件失败: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Println("配置文件:", path)

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

	count, err := followrepo.New(db, redisClient).RestoreSets(context.Background())
	if err != nil {
		log.Fatalf("重建关注集合失败: %v", err)
	}
	log.Printf("重建完成，处理关注关系 %d 条", count)
}

// resolveConfigPath 从当前工作目录逐级向上查找配置文件
func resolveConfigPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("获取绝对路径失败: %w", err)
	}
	for {
		candidate := filepath.Join(dir, config.DefaultRelativePath)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("没有找到 %s", config.DefaultRelativePath)
}
