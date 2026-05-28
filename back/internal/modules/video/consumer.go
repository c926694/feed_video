package video

import (
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
)

func (m *Module) RegisterConsumers(registrar modulekit.ConsumerRegistrar) error {
	if err := m.videoConsumer.Declare(event.DeleteVideoExchange, event.DeleteVideoExchangeType,
		event.DeleteVideoQueue, event.DeleteVideoRoutingKey); err != nil {
		return err
	}
	registrar.Add("video.delete", func() error {
		return m.videoConsumer.ListenVideoConsumer(event.DeleteVideoQueue, m.videoConsumer.DeleteVideoHandler)
	})
	return nil
}
