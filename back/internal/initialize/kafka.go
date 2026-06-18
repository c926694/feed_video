package initialize

import (
	"fmt"

	"github.com/segmentio/kafka-go"
)

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
}

var KafkaWriter *kafka.Writer

func InitKafka(cfg KafkaConfig) (*kafka.Writer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers is empty")
	}

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}

	KafkaWriter = writer
	return writer, nil
}

func CloseKafka() {
	if KafkaWriter != nil {
		_ = KafkaWriter.Close()
	}
}
