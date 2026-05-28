package feed

import (
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
	consumer2 "simple_tiktok/internal/mq/consumer"
	mysqlrepo "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/gin-gonic/gin"
)

type Module struct {
	videoHotConsumer *consumer2.VideoHotConsumer
}

func NewModule(ctx modulekit.Context) (*Module, error) {
	videoRepo := mysqlrepo.NewVideoRepo(ctx.DB)
	userRepo := mysqlrepo.NewUserRepo(ctx.DB)
	commentRepo := mysqlrepo.NewCommentRepo(ctx.DB)
	serviceChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoService := service.NewVideoService(videoRepo, userRepo, ctx.Redis, serviceChannel, commentRepo)

	consumerChannel, err := ctx.RabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	videoHotConsumer := consumer2.NewVideoHotConsumer(consumerChannel, videoRepo, videoService)
	return &Module{
		videoHotConsumer: videoHotConsumer,
	}, nil
}

func (m *Module) RegisterHTTP(r *gin.Engine) error {
	return nil
}

func (m *Module) RegisterConsumers(registrar modulekit.ConsumerRegistrar) error {
	if err := m.videoHotConsumer.Declare(event.VideoHotExchange, event.VideoHotExchangeType,
		event.VideoHotQueue, event.VideoHotRoutingKey); err != nil {
		return err
	}
	registrar.Add("feed.hot", func() error {
		return m.videoHotConsumer.Listen(event.VideoHotQueue, m.videoHotConsumer.HotUpdateHandler)
	})
	return nil
}
