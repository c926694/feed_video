package like

import (
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
)

func (m *Module) RegisterConsumers(registrar modulekit.ConsumerRegistrar) error {
	if err := m.likeConsumer.Declare(event.LikeVideoExchange, event.LikeVideoExchangeType,
		event.LikeVideoQueue, event.LikeVideoRoutingKey); err != nil {
		return err
	}
	if err := m.likeConsumer.Declare(event.LikeCommentExchange, event.LikeCommentExchangeType,
		event.LikeCommentQueue, event.LikeCommentRoutingKey); err != nil {
		return err
	}
	registrar.Add("like.video", func() error {
		return m.likeConsumer.ListenLikeConsumer(event.LikeVideoQueue, m.likeConsumer.LikeVideoHandler)
	})
	registrar.Add("like.comment", func() error {
		return m.likeConsumer.ListenLikeConsumer(event.LikeCommentQueue, m.likeConsumer.LikeCommentHandler)
	})
	return nil
}
