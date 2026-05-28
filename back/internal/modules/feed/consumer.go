package feed

import (
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
)

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
