package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	publishRetryTimes   = 3
	publishRetryBackoff = 200 * time.Millisecond
	writeBatchTimeout   = 10 * time.Millisecond
	ensureTopicsTimeout = 10 * time.Second
)

// Publisher 事件发布器。按 topic 复用 Writer，内部完成序列化、topic 创建与发送重试。
type Publisher struct {
	brokers []string
	mu      sync.Mutex
	writers map[string]*kafka.Writer
	ensured map[string]bool
}

// NewPublisher 创建发布器。brokers 为空属于配置错误，直接返回错误。
func NewPublisher(brokers []string) (*Publisher, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers 为空")
	}
	return &Publisher{
		brokers: brokers,
		writers: make(map[string]*kafka.Writer),
		ensured: make(map[string]bool),
	}, nil
}

// EnsureTopics 尝试确保全部业务 topic 已经存在。单次失败只记录日志，
// 进程照常提供服务，真正写消息时会再次暴露问题。
func (p *Publisher) EnsureTopics(ctx context.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx, ensureTopicsTimeout)
	defer cancel()
	for _, topic := range allTopics {
		if err := p.EnsureTopic(timeoutCtx, topic); err != nil {
			slog.Error("确保 topic 存在失败", "topic", topic, "error", err)
		}
	}
}

// EnsureTopic 确保 topic 存在，已经存在时跳过，同一个 topic 只检查一次
func (p *Publisher) EnsureTopic(ctx context.Context, topic string) error {
	p.mu.Lock()
	done := p.ensured[topic]
	p.mu.Unlock()
	if done {
		return nil
	}

	conn, err := p.dialAny(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("获取 kafka controller 失败: %w", err)
	}
	controllerConn, err := kafka.DialContext(ctx, "tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("连接 kafka controller 失败: %w", err)
	}
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	if err != nil && !errors.Is(err, kafka.TopicAlreadyExists) {
		return fmt.Errorf("创建 topic %s 失败: %w", topic, err)
	}

	p.mu.Lock()
	p.ensured[topic] = true
	p.mu.Unlock()
	return nil
}

// Publish 把 payload 序列化后发送到 topic，失败时带退避重试
func (p *Publisher) Publish(ctx context.Context, topic string, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 %s 的消息失败: %w", topic, err)
	}
	return p.publishRaw(ctx, topic, key, data)
}

// Close 关闭全部 Writer
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var lastErr error
	for topic, writer := range p.writers {
		if err := writer.Close(); err != nil {
			lastErr = fmt.Errorf("关闭 %s 的 writer 失败: %w", topic, err)
		}
	}
	return lastErr
}

// publishRaw 发送已经序列化好的消息
func (p *Publisher) publishRaw(ctx context.Context, topic string, key string, value []byte) error {
	writer := p.writerFor(topic)
	var lastErr error
	for attempt := 0; attempt < publishRetryTimes; attempt++ {
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
		time.Sleep(time.Duration(attempt+1) * publishRetryBackoff)
	}
	return fmt.Errorf("发送 %s 的消息失败: %w", topic, lastErr)
}

func (p *Publisher) writerFor(topic string) *kafka.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()
	if writer, ok := p.writers[topic]; ok {
		return writer
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(p.brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		BatchTimeout: writeBatchTimeout,
	}
	p.writers[topic] = writer
	return writer
}

// dialAny 依次尝试全部 broker，返回第一个连接成功的连接
func (p *Publisher) dialAny(ctx context.Context) (*kafka.Conn, error) {
	var lastErr error
	for _, broker := range p.brokers {
		conn, err := kafka.DialContext(ctx, "tcp", broker)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("连接全部 kafka broker 失败: %w", lastErr)
}
