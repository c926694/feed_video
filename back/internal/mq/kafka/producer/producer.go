package producer

import (
	"context"
	"errors"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	Writer *kafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		Writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafka.LeastBytes{},
			RequiredAcks:           kafka.RequireOne,
			Async:                  false,
			BatchTimeout:           10 * time.Millisecond,
			AllowAutoTopicCreation: true,
		},
	}
}

func (p *Producer) SendMessage(ctx context.Context, key string, value []byte) error {
	retry := 3
	for i := 0; i < retry; i++ {
		if err := p.Writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(key),
			Value: value,
			Time:  time.Now(),
		}); err != nil && errors.Is(err, kafka.LeaderNotAvailable) {
			time.Sleep(250 * time.Millisecond)
			continue
		} else {
			break
		}
	}
	return nil
}

func (p *Producer) Close() error {
	return p.Writer.Close()
}
