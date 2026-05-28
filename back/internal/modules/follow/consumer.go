package follow

import (
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
)

func (m *Module) RegisterConsumers(registrar modulekit.ConsumerRegistrar) error {
	if err := m.followConsumer.Declare(event.FollowExchange, event.FollowExchangeType,
		event.FollowQueue, event.FollowRoutingKey); err != nil {
		return err
	}
	registrar.Add("follow.switch", func() error {
		return m.followConsumer.ListenFollowConsumer(event.FollowQueue, m.followConsumer.FollowHandler)
	})
	return nil
}
