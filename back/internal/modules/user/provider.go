package user

import (
	"simple_tiktok/internal/modulekit"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	httpHandler *HTTPHandler
	redis      *redis.Client
}

func NewModule(ctx modulekit.Context) *Module {
	userRepo := NewUserRepo(ctx.DB)
	videoRepo := NewVideoRepo(ctx.DB)
	userService := NewService(userRepo, videoRepo, ctx.Redis)
	return &Module{
		httpHandler: NewHTTPHandler(userService),
		redis:      ctx.Redis,
	}
}
