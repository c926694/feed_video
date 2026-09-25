package consumer

import (
	"context"
	"log/slog"
	"reflect"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/segmentio/kafka-go"
	"golang.org/x/sync/errgroup"

	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
)

const (
	retryTimes       = 3
	retryBackoff     = 300 * time.Millisecond
	readRetryBackoff = time.Second
	minBytes         = 1
	maxBytes         = 10e6
	maxWait          = 200 * time.Millisecond
)

// Handler 处理一条消息，payload 是消息体的原始字节，由处理函数自己解码
type Handler func(ctx context.Context, payload []byte) error

type subscription struct {
	name    string
	handler Handler
}

// handlerName 取处理函数的完整名称，用于日志里区分同一个 topic 上的多个订阅
func (s subscription) handlerName() string {
	fn := runtime.FuncForPC(reflect.ValueOf(s.handler).Pointer())
	if fn == nil {
		return "未知处理函数"
	}
	return fn.Name()
}

// Consumer 事件消费者。负责读取消息、提交位点、失败重试、转发失败消息与 panic 恢复。
// dlqProducer 只用于把处理失败的消息转发到 "<topic>.failed"。
type Consumer struct {
	brokers     []string
	groupID     string
	subscriber  string
	dlqProducer *producer.Producer
	entries     []subscription
}

// New 创建消费者。groupID 是消费者组前缀，subscriber 是登记订阅的模块名。
// 每个订阅使用 "前缀.模块名.topic" 作为组名，因此同一个模块可以订阅多个 topic，
// 多个模块也可以订阅同一个 topic，各自都能收到全部消息。
func New(brokers []string, groupID string, subscriber string, dlqProducer *producer.Producer) *Consumer {
	return &Consumer{
		brokers:     brokers,
		groupID:     groupID,
		subscriber:  subscriber,
		dlqProducer: dlqProducer,
	}
}

// Subscribe 登记一个订阅
func (c *Consumer) Subscribe(name string, handler Handler) {
	c.entries = append(c.entries, subscription{name: name, handler: handler})
}

// Run 为每个订阅启动一个消费循环，全部结束后返回
func (c *Consumer) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, entry := range c.entries {
		e := entry
		g.Go(func() error {
			return c.consume(gctx, e)
		})
	}
	return g.Wait()
}

func (c *Consumer) consume(ctx context.Context, entry subscription) error {
	group := c.groupID + "." + c.subscriber + "." + entry.name
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     c.brokers,
		Topic:       entry.name,
		GroupID:     group,
		MinBytes:    minBytes,
		MaxBytes:    maxBytes,
		StartOffset: kafka.FirstOffset,
		MaxWait:     maxWait,
	})
	defer reader.Close()

	slog.Info("开始消费", "topic", entry.name, "group", group)

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// 读取失败（例如 broker 暂时连不上）不结束消费循环，否则会连带停掉整个进程
			slog.Error("读取消息失败，稍后重试", "topic", entry.name, "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(readRetryBackoff):
			}
			continue
		}
		c.dispatch(ctx, entry, msg)
		if err := reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("提交位点失败，稍后重试", "topic", entry.name, "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(readRetryBackoff):
			}
		}
	}
}

// dispatch 执行处理函数。可重试的错误带退避重试，仍失败则把原始消息转发到失败 topic，
// 处理函数里的 panic 就地恢复，两种情况都提交位点继续处理后续消息。
func (c *Consumer) dispatch(ctx context.Context, entry subscription, msg kafka.Message) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("处理消息时发生 panic",
				"topic", entry.name,
				"partition", msg.Partition,
				"offset", msg.Offset,
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()

	var lastErr error
	for attempt := 0; attempt < retryTimes; attempt++ {
		lastErr = entry.handler(ctx, msg.Value)
		if lastErr == nil {
			slog.Info("消息处理完成",
				"topic", entry.name,
				"handler", entry.handlerName(),
				"partition", msg.Partition,
				"offset", msg.Offset,
				"attempt", attempt+1)
			return
		}
		if IsPermanent(lastErr) {
			break
		}
		if ctx.Err() != nil {
			return
		}
		time.Sleep(time.Duration(attempt+1) * retryBackoff)
	}

	slog.Error("消息处理失败",
		"topic", entry.name,
		"partition", msg.Partition,
		"offset", msg.Offset,
		"error", lastErr)

	failedTopic := entry.name + topic.FailedSuffix
	if err := c.dlqProducer.PublishRaw(ctx, failedTopic, string(msg.Key), msg.Value); err != nil {
		slog.Error("转发失败消息失败", "topic", failedTopic, "offset", msg.Offset, "error", err)
	}
}
