package user

import (
	"simple_tiktok/internal/controller"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/modulekit"
	mysqlrepo "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type Module struct {
	controller *controller.UserController
	redis      *redis.Client
}

func NewModule(ctx modulekit.Context) *Module {
	userRepo := mysqlrepo.NewUserRepo(ctx.DB)
	videoRepo := mysqlrepo.NewVideoRepo(ctx.DB)
	userService := service.NewUserService(userRepo, videoRepo, ctx.Redis)
	return &Module{
		controller: controller.NewUserController(userService),
		redis:      ctx.Redis,
	}
}

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	userGroup := r.Group("users")
	{
		userGroup.POST("/login", m.controller.Login)
		userGroup.POST("/register", m.controller.Register)
		userGroup.DELETE("/logout", middleware.JWTAuth(m.redis), m.controller.Logout)
		userGroup.GET("/me", middleware.JWTAuth(m.redis), m.controller.GetUserInfo)
		userGroup.POST("/me", middleware.JWTAuth(m.redis), m.controller.UpdateProfile)
	}
	return nil
}

func (m *Module) RegisterConsumers(registrar modulekit.ConsumerRegistrar) error {
	return nil
}
