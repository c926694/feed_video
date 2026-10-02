package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	messagerepo "simple_tiktok/internal/modules/message/repo"
	"simple_tiktok/internal/platform/config"
	"simple_tiktok/internal/platform/mysql"
)

// 清理已读且长期没有新互动的通知会话，再删它们的明细。
// 未读的通知不清理
func main() {
	days := flag.Int("days", 90, "保留天数，删掉已读且超过这么多天没有新互动的会话")
	flag.Parse()

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

	before := time.Now().AddDate(0, 0, -*days)
	count, err := messagerepo.New(db).Cleanup(context.Background(), before, 500)
	if err != nil {
		log.Fatalf("清理通知失败: %v", err)
	}
	log.Printf("清理完成，删除会话 %d 条（保留 %d 天内的已读通知）", count, *days)
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
