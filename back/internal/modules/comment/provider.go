package comment

import (
	"simple_tiktok/internal/controller"
	"simple_tiktok/internal/modulekit"
	mysqlrepo "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/redis/go-redis/v9"
)

type Module struct {
	controller *controller.CommentController
	redis      *redis.Client
}

func NewModule(ctx modulekit.Context) (*Module, error) {
	commentRepo := mysqlrepo.NewCommentRepo(ctx.DB)
	videoRepo := mysqlrepo.NewVideoRepo(ctx.DB)
	userRepo := mysqlrepo.NewUserRepo(ctx.DB)
	hotMQ, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	commentService := service.NewCommentService(commentRepo, videoRepo, userRepo, ctx.Redis, hotMQ)
	return &Module{
		controller: controller.NewCommentController(commentService),
		redis:      ctx.Redis,
	}, nil
}
