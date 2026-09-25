package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	retryTimes   = 3
	retryBackoff = 200 * time.Millisecond
	batchTimeout = 10 * time.Millisecond
)

// Producer 事件生产者。按 topic 复用 Writer，内部完成序列化与发送重试。
// topic 不在这里创建，交给 broker 的自动创建（auto.create.topics.enable）。
type Producer struct {
	brokers []string
	mu      sync.Mutex
	writers map[string]*kafka.Writer
}

// New 创建生产者。brokers 为空属于配置错误，直接返回错误。
func New(brokers []string) (*Producer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers 为空")
	}
	return &Producer{
		brokers: brokers,
		writers: make(map[string]*kafka.Writer),
	}, nil
}

// Publish 把 payload 序列化后发送到指定 topic，失败时带退避重试
func (p *Producer) Publish(ctx context.Context, name string, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 %s 的消息失败: %w", name, err)
	}
	return p.PublishRaw(ctx, name, key, data)
}

// PublishRaw 发送已经序列化好的消息，供转发失败消息这类场景使用
func (p *Producer) PublishRaw(ctx context.Context, name string, key string, value []byte) error {
	writer := p.writerFor(name)
	var lastErr error
	for attempt := 0; attempt < retryTimes; attempt++ {
		lastErr = writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(key),
			Value: value,
			Time:  time.Now(),
		})
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return lastErr
		}
		time.Sleep(time.Duration(attempt+1) * retryBackoff)
	}
	return fmt.Errorf("发送 %s 的消息失败: %w", name, lastErr)
}

// Close 关闭全部 Writer
func (p *Producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var lastErr error
	for name, writer := range p.writers {
		if err := writer.Close(); err != nil {
			lastErr = fmt.Errorf("关闭 %s 的 writer 失败: %w", name, err)
		}
	}
	return lastErr
}

func (p *Producer) writerFor(name string) *kafka.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()
	if writer, ok := p.writers[name]; ok {
		return writer
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(p.brokers...),
		Topic:        name,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		BatchTimeout: batchTimeout,
	}
	p.writers[name] = writer
	return writer
}