package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"runtime/debug"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"

	"simple_tiktok/internal/platform/mq/producer"
	"simple_tiktok/internal/platform/mq/topic"
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

// Consumer 事件消费者。一个模块一个 PushConsumer，组名是 "<前缀>_<模块名>"，
// 同一组内的多个实例分摊消息，不同组各自收到全部消息。
// dlqProducer 只用于把不可重试的消息转发到 "<topic>.failed"。
type Consumer struct {
	endpoint     string
	accessKey    string
	accessSecret string
	groupID      string
	subscriber   string
	dlqProducer  *producer.Producer
	entries      []subscription
}

// New 创建消费者。subscriber 是登记订阅的模块名，组名由前缀与模块名拼成。
func New(endpoint string, accessKey string, accessSecret string, groupID string, subscriber string, dlqProducer *producer.Producer) *Consumer {
	return &Consumer{
		endpoint:     endpoint,
		accessKey:    accessKey,
		accessSecret: accessSecret,
		groupID:      groupID,
		subscriber:   subscriber,
		dlqProducer:  dlqProducer,
	}
}

// Subscribe 登记一个订阅
func (c *Consumer) Subscribe(name string, handler Handler) {
	c.entries = append(c.entries, subscription{name: name, handler: handler})
}

// Run 启动消费，直到 ctx 被取消后优雅关闭。同一个实例订阅的 topic 集合固定，
// 因此一个模块的所有订阅必须在 Start 之前登记完。
func (c *Consumer) Run(ctx context.Context) error {
	if len(c.entries) == 0 {
		return nil
	}
	group := c.groupID + "_" + c.subscriber
	expressions := make(map[string]*rmq.FilterExpression, len(c.entries))
	byTopic := make(map[string]subscription, len(c.entries))
	for _, entry := range c.entries {
		if _, ok := byTopic[entry.name]; ok {
			return fmt.Errorf("组 %s 重复订阅 %s", group, entry.name)
		}
		expressions[entry.name] = rmq.SUB_ALL
		byTopic[entry.name] = entry
	}

	push, err := rmq.NewPushConsumer(&rmq.Config{
		Endpoint:      c.endpoint,
		ConsumerGroup: group,
		Credentials: &credentials.SessionCredentials{
			AccessKey:    c.accessKey,
			AccessSecret: c.accessSecret,
		},
	},
		rmq.WithPushSubscriptionExpressions(expressions),
		rmq.WithPushMessageListener(&rmq.FuncMessageListener{
			Consume: func(view *rmq.MessageView) rmq.ConsumerResult {
				return c.dispatch(ctx, group, byTopic, view)
			},
		}),
	)
	if err != nil {
		return fmt.Errorf("创建 rocketmq 消费者失败: %w", err)
	}
	if err = push.Start(); err != nil {
		return fmt.Errorf("启动 rocketmq 消费者失败: %w", err)
	}
	slog.Info("开始消费", "group", group, "topics", len(c.entries))

	<-ctx.Done()
	if err = push.GracefulStop(); err != nil {
		return fmt.Errorf("关闭 rocketmq 消费者失败: %w", err)
	}
	return nil
}

// dispatch 执行处理函数。不可重试的错误把原始消息转发到失败 topic 后确认消费，
// 其余错误返回 FAILURE 交给服务端重投，重投耗尽后进入 "<分组>%DLQ%"。
// 处理函数里的 panic 就地恢复，避免整组消费停摆。
func (c *Consumer) dispatch(ctx context.Context, group string, byTopic map[string]subscription, view *rmq.MessageView) (result rmq.ConsumerResult) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("处理消息时发生 panic",
				"topic", view.GetTopic(),
				"message_id", view.GetMessageId(),
				"panic", r,
				"stack", string(debug.Stack()))
			result = rmq.SUCCESS
		}
	}()

	entry, ok := byTopic[view.GetTopic()]
	if !ok {
		slog.Error("收到未登记的 topic，直接确认", "topic", view.GetTopic(), "message_id", view.GetMessageId())
		return rmq.SUCCESS
	}

	err := entry.handler(ctx, view.GetBody())
	if err == nil {
		slog.Info("消息处理完成",
			"topic", entry.name,
			"handler", entry.handlerName(),
			"message_id", view.GetMessageId(),
			"attempt", view.GetDeliveryAttempt())
		return rmq.SUCCESS
	}

	if IsPermanent(err) {
		slog.Error("消息不可重试，转发到失败 topic",
			"topic", entry.name,
			"message_id", view.GetMessageId(),
			"error", err)
		key := ""
		if keys := view.GetKeys(); len(keys) > 0 {
			key = keys[0]
		}
		failedTopic := topic.FailedTopic(entry.name)
		if pubErr := c.dlqProducer.PublishRaw(ctx, failedTopic, key, view.GetBody()); pubErr != nil {
			slog.Error("转发失败消息失败", "topic", failedTopic, "error", pubErr)
		}
		return rmq.SUCCESS
	}

	slog.Error("消息处理失败，等待服务端重投",
		"topic", entry.name,
		"handler", entry.handlerName(),
		"message_id", view.GetMessageId(),
		"group", group,
		"attempt", view.GetDeliveryAttempt(),
		"error", err)
	return rmq.FAILURE
}
