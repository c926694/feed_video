package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"

	"simple_tiktok/internal/modules/comment"
	"simple_tiktok/internal/modules/feed"
	"simple_tiktok/internal/modules/follow"
	"simple_tiktok/internal/modules/like"
	"simple_tiktok/internal/modules/user"
	"simple_tiktok/internal/modules/video"
	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/config"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/mysql"
	"simple_tiktok/internal/platform/redis"
	"simple_tiktok/internal/platform/upload"
	"simple_tiktok/internal/svc"
)

const shutdownTimeout = 15 * time.Second

func main() {
	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	resolvedPath, err := resolveConfigPath()
	if err != nil {
		log.Fatalf("定位配置文件失败: %v", err)
	}
	cfg, err := config.Load(resolvedPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Println("配置文件:", resolvedPath)

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

	eventProducer, err := producer.New(cfg.Kafka.Brokers)
	if err != nil {
		log.Fatalf("初始化 kafka 生产者失败: %v", err)
	}

	authService, err := auth.New(cfg.JWT.Secret, cfg.JWT.ExpireHours, redisClient)
	if err != nil {
		log.Fatalf("初始化鉴权失败: %v", err)
	}

	uploader, err := upload.New(cfg.Upload)
	if err != nil {
		log.Fatalf("初始化上传目录失败: %v", err)
	}

	// 建立或更新表结构，并补齐历史数据里新增的冗余列
	if err = mysql.Migrate(db); err != nil {
		log.Fatalf("建立表结构失败: %v", err)
	}

	serviceCtx := &svc.ServiceContext{
		DB:       db,
		Redis:    redisClient,
		Producer: eventProducer,
		Auth:     authService,
		Upload:   uploader,
	}

	gin.SetMode(cfg.Server.Mode)
	r := gin.Default()
	// 允许前端开发服务器跨域访问
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.Server.AllowOrigins,
		AllowWildcard:    true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	r.HandleMethodNotAllowed = true
	r.NoRoute(func(c *gin.Context) {
		httpx.Fail(c, httpx.New(httpx.CodeNotFound, "接口不存在"))
	})
	r.NoMethod(func(c *gin.Context) {
		httpx.Fail(c, httpx.New(httpx.CodeMethodNotAllowed, "请求方法不允许"))
	})

	// 模块自己在模块内部完成装配，这里只负责传入进程级的共享依赖
	httpRegistrations := []func(*gin.Engine, *svc.ServiceContext) (*gin.Engine, error){
		user.RegisterHTTP,
		like.RegisterHTTP,
		follow.RegisterHTTP,
		video.RegisterHTTP,
		comment.RegisterHTTP,
		feed.RegisterHTTP,
	}
	for _, register := range httpRegistrations {
		if _, err = register(r, serviceCtx); err != nil {
			log.Fatalf("注册路由失败: %v", err)
		}
	}

	// 每个模块一个消费者，组名里带上模块名。多个模块订阅同一个 topic 时，
	// 分处不同的消费组，各自都能收到全部消息。
	var consumers []*consumer.Consumer
	consumerRegistrations := []struct {
		subscriber string
		register   func(*consumer.Consumer, *svc.ServiceContext) error
	}{
		{user.Name, user.RegisterConsumers},
		{follow.Name, follow.RegisterConsumers},
		{video.Name, video.RegisterConsumers},
		{comment.Name, comment.RegisterConsumers},
		{feed.Name, feed.RegisterConsumers},
	}
	for _, item := range consumerRegistrations {
		eventConsumer := consumer.New(cfg.Kafka.Brokers, cfg.Kafka.GroupID, item.subscriber, eventProducer)
		if err = item.register(eventConsumer, serviceCtx); err != nil {
			log.Fatalf("注册消费者失败: %v", err)
		}
		consumers = append(consumers, eventConsumer)
	}
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	log.Println("监听端口:", addr)

	g, gctx := errgroup.WithContext(runCtx)
	g.Go(func() error {
		if serveErr := server.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			return fmt.Errorf("启动 http 服务失败: %w", serveErr)
		}
		return nil
	})
	for _, item := range consumers {
		eventConsumer := item
		g.Go(func() error {
			return eventConsumer.Run(gctx)
		})
	}
	g.Go(func() error {
		<-gctx.Done()
		log.Println("收到停止信号，正在关闭服务")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("关闭 http 服务失败: %w", shutdownErr)
		}
		return nil
	})

	if err = g.Wait(); err != nil {
		log.Printf("服务退出: %v", err)
	}
	if closeErr := eventProducer.Close(); closeErr != nil {
		log.Printf("关闭 kafka 生产者失败: %v", closeErr)
	}
	log.Println("服务已停止")
}

// resolveConfigPath 取当前工作目录的绝对路径，再从那里逐级向上查找配置文件。
// 从 back、back/cmd/server 或容器里的 /app 启动都能找到同一份配置。
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
