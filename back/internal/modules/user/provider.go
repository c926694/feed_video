package user

import (
	"simple_tiktok/internal/modulekit"
	mysqlrepo "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	httpHandler *HTTPHandler
	redis      *redis.Client
}

func NewModule(ctx modulekit.Context) *Module {
	userRepo := mysqlrepo.NewUserRepo(ctx.DB)
	videoRepo := mysqlrepo.NewVideoRepo(ctx.DB)
	userService := service.NewUserService(userRepo, videoRepo, ctx.Redis)
	return &Module{
		httpHandler: NewHTTPHandler(userService),
		redis:      ctx.Redis,
	}
}
