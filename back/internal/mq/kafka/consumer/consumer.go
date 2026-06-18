package consumer

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type MessageHandler func(ctx context.Context, msg kafka.Message) error

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(brokers []string, topic string, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     groupID,
			MinBytes:    1,
			MaxBytes:    10e6,
			StartOffset: kafka.FirstOffset,
			MaxWait:     200 * time.Millisecond,
		}),
	}
}

func (c *Consumer) Consume(ctx context.Context, handler MessageHandler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		if err := handler(ctx, msg); err != nil {
			log.Printf("消费消息失败:%v,%v", msg, err)
			return err
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
