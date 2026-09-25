package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/segmentio/kafka-go"
	"golang.org/x/sync/errgroup"
)

const (
	handlerRetryTimes   = 3
	handlerRetryBackoff = 300 * time.Millisecond
	consumeMinBytes     = 1
	consumeMaxBytes     = 10e6
	consumeMaxWait      = 200 * time.Millisecond
)

// Handler 处理一条消息，payload 是消息体的原始字节，由处理函数自己解码
type Handler func(ctx context.Context, payload []byte) error

type subscription struct {
	topic   string
	handler Handler
}

// Subscriber 事件订阅器。负责读取消息、提交位点、失败重试、转发失败消息与 panic 恢复。
type Subscriber struct {
	publisher *Publisher
	groupID   string
	entries   []subscription
}

// NewSubscriber 创建订阅器，groupID 是消费者组前缀，每个 topic 会追加自己的名称
func NewSubscriber(publisher *Publisher, groupID string) *Subscriber {
	return &Subscriber{
		publisher: publisher,
		groupID:   groupID,
	}
}

// Subscribe 登记一个订阅
func (s *Subscriber) Subscribe(topic string, handler Handler) {
	s.entries = append(s.entries, subscription{topic: topic, handler: handler})
}

// Run 为每个订阅启动一个消费循环，全部结束后返回
func (s *Subscriber) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, entry := range s.entries {
		e := entry
		g.Go(func() error {
			return s.consume(gctx, e)
		})
	}
	return g.Wait()
}

func (s *Subscriber) consume(ctx context.Context, entry subscription) error {
	group := s.groupID + "." + entry.topic
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     s.publisher.brokers,
		Topic:       entry.topic,
		GroupID:     group,
		MinBytes:    consumeMinBytes,
		MaxBytes:    consumeMaxBytes,
		StartOffset: kafka.FirstOffset,
		MaxWait:     consumeMaxWait,
	})
	defer reader.Close()

	slog.Info("开始消费", "topic", entry.topic, "group", group)

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("读取 %s 的消息失败: %w", entry.topic, err)
		}
		s.dispatch(ctx, entry, msg)
		if err := reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("提交 %s 的位点失败: %w", entry.topic, err)
		}
	}
}

// dispatch 执行处理函数。可重试的错误带退避重试，仍失败则把原始消息转发到失败 topic，
// 处理函数里的 panic 就地恢复，两种情况都提交位点继续处理后续消息。
func (s *Subscriber) dispatch(ctx context.Context, entry subscription, msg kafka.Message) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("处理消息时发生 panic",
				"topic", entry.topic,
				"partition", msg.Partition,
				"offset", msg.Offset,
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()

	var lastErr error
	for attempt := 0; attempt < handlerRetryTimes; attempt++ {
		lastErr = entry.handler(ctx, msg.Value)
		if lastErr == nil {
			return
		}
		if IsPermanent(lastErr) {
			break
		}
		if ctx.Err() != nil {
			return
		}
		time.Sleep(time.Duration(attempt+1) * handlerRetryBackoff)
	}

	slog.Error("消息处理失败",
		"topic", entry.topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
		"error", lastErr)

	failedTopic := entry.topic + FailedSuffix
	if err := s.publisher.publishRaw(ctx, failedTopic, string(msg.Key), msg.Value); err != nil {
		slog.Error("转发失败消息失败", "topic", failedTopic, "offset", msg.Offset, "error", err)
	}
}
