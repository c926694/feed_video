package comment

import (
	"simple_tiktok/internal/svc"
	"simple_tiktok/internal/service"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	httpHandler *HTTPHandler
	redis      *redis.Client
}

func NewModule(ctx *svc.ServiceContext) (*Module, error) {
	commentRepo := NewCommentRepo(ctx.DB)
	videoRepo := NewVideoRepo(ctx.DB)
	userRepo := NewUserRepo(ctx.DB)
	hotMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	feedService := service.NewFeedService(videoRepo, userRepo, ctx.Redis, hotMQ)
	commentService := NewService(commentRepo, videoRepo, userRepo, ctx.Redis, feedService)
	return &Module{
		httpHandler: NewHTTPHandler(commentService),
		redis:      ctx.Redis,
	}, nil
}
