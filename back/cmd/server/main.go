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
	"golang.org/x/sync/errgroup"

	"simple_tiktok/internal/modules/comment"
	commentrepo "simple_tiktok/internal/modules/comment/repo"
	"simple_tiktok/internal/modules/feed"
	feedrepo "simple_tiktok/internal/modules/feed/repo"
	"simple_tiktok/internal/modules/follow"
	followrepo "simple_tiktok/internal/modules/follow/repo"
	"simple_tiktok/internal/modules/like"
	likerepo "simple_tiktok/internal/modules/like/repo"
	"simple_tiktok/internal/modules/user"
	userrepo "simple_tiktok/internal/modules/user/repo"
	"simple_tiktok/internal/modules/video"
	videorepo "simple_tiktok/internal/modules/video/repo"
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

	eventProducer, err := producer.New(cfg.Kafka.Brokers)
	if err != nil {
		log.Fatalf("初始化 kafka 生产者失败: %v", err)
	}
	eventProducer.EnsureTopics(runCtx)

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

	users := userrepo.New(db)
	videos := videorepo.New(db, redisClient)
	comments := commentrepo.New(db)
	follows := followrepo.New(db, redisClient)
	likes := likerepo.New(redisClient)
	feeds := feedrepo.New(redisClient)

	// 业务层，跨模块的数据访问直接依赖对方的 repo
	userModel := user.NewModel(users, authService, uploader, eventProducer)
	likeModel := like.NewModel(likes, eventProducer)
	followModel := follow.NewModel(follows, eventProducer)
	videoModel := video.NewModel(videos, users, likes, follows, eventProducer, uploader)
	commentModel := comment.NewModel(comments, users, likes, eventProducer, uploader)
	feedModel := feed.NewModel(feeds, videos, users, likes, follows, uploader)

	serviceCtx := &svc.ServiceContext{
		DB:       db,
		Redis:    redisClient,
		Producer: eventProducer,
		Auth:     authService,
		Upload:   uploader,
	}

	gin.SetMode(cfg.Server.Mode)
	r := gin.Default()
	r.HandleMethodNotAllowed = true
	r.NoRoute(func(c *gin.Context) {
		httpx.Fail(c, httpx.New(httpx.CodeNotFound, "接口不存在"))
	})
	r.NoMethod(func(c *gin.Context) {
		httpx.Fail(c, httpx.New(httpx.CodeMethodNotAllowed, "请求方法不允许"))
	})

	httpRegistrations := []func(*gin.Engine, *svc.ServiceContext) (*gin.Engine, error){
		func(engine *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
			return user.RegisterHTTP(engine, userModel, ctx)
		},
		func(engine *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
			return like.RegisterHTTP(engine, likeModel, ctx)
		},
		func(engine *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
			return follow.RegisterHTTP(engine, followModel, ctx)
		},
		func(engine *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
			return video.RegisterHTTP(engine, videoModel, ctx)
		},
		func(engine *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
			return comment.RegisterHTTP(engine, commentModel, ctx)
		},
		func(engine *gin.Engine, ctx *svc.ServiceContext) (*gin.Engine, error) {
			return feed.RegisterHTTP(engine, feedModel, ctx)
		},
	}
	for _, register := range httpRegistrations {
		if _, err = register(r, serviceCtx); err != nil {
			log.Fatalf("注册路由失败: %v", err)
		}
	}

	eventConsumer := consumer.New(cfg.Kafka.Brokers, cfg.Kafka.GroupID, eventProducer)
	consumerRegistrations := []func(*consumer.Consumer, *svc.ServiceContext) error{
		func(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
			return user.RegisterConsumers(sub, userModel, ctx)
		},
		func(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
			return follow.RegisterConsumers(sub, followModel, ctx)
		},
		func(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
			return video.RegisterConsumers(sub, videoModel, ctx)
		},
		func(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
			return comment.RegisterConsumers(sub, commentModel, ctx)
		},
		func(sub *consumer.Consumer, ctx *svc.ServiceContext) error {
			return feed.RegisterConsumers(sub, feedModel, ctx)
		},
	}
	for _, register := range consumerRegistrations {
		if err = register(eventConsumer, serviceCtx); err != nil {
			log.Fatalf("注册消费者失败: %v", err)
		}
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
	g.Go(func() error {
		return eventConsumer.Run(gctx)
	})
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
