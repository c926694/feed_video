package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
)

const (
	retryTimes   = 3
	retryBackoff = 200 * time.Millisecond
)

// Producer 事件生产者。整个进程共用一个客户端，topic 在服务端预先创建。
// 发送时按 topic 查路由，没有可用队列会返回错误，因此不支持向未创建的 topic 发送。
type Producer struct {
	client rmq.Producer
}

// New 创建并启动生产者。endpoint 是 Proxy 的 gRPC 地址。
// topics 必须给出全部要发送的 topic：客户端在 Start 时按这个列表拉取路由，
// 列表为空会导致同步 settings 永远等不到结果。
func New(endpoint string, accessKey string, accessSecret string, topics []string) (*Producer, error) {
	if endpoint == "" {
		return nil, errors.New("rocketmq endpoint 为空")
	}
	if len(topics) == 0 {
		return nil, errors.New("rocketmq topic 列表为空")
	}
	// 本地部署没有开启 TLS，而客户端默认要求 TLS，这里显式关闭
	rmq.EnableSsl = false
	client, err := rmq.NewProducer(&rmq.Config{
		Endpoint: endpoint,
		Credentials: &credentials.SessionCredentials{
			AccessKey:    accessKey,
			AccessSecret: accessSecret,
		},
	}, rmq.WithTopics(topics...))
	if err != nil {
		return nil, fmt.Errorf("创建 rocketmq 生产者失败: %w", err)
	}
	if err = client.Start(); err != nil {
		return nil, fmt.Errorf("启动 rocketmq 生产者失败: %w", err)
	}
	return &Producer{client: client}, nil
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
	var lastErr error
	for attempt := 0; attempt < retryTimes; attempt++ {
		msg := &rmq.Message{Topic: name, Body: value}
		if key != "" {
			msg.SetKeys(key)
		}
		if _, lastErr = p.client.Send(ctx, msg); lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return lastErr
		}
		time.Sleep(time.Duration(attempt+1) * retryBackoff)
	}
	return fmt.Errorf("发送 %s 的消息失败: %w", name, lastErr)
}

// Close 关闭生产者
func (p *Producer) Close() error {
	if err := p.client.GracefulStop(); err != nil {
		return fmt.Errorf("关闭 rocketmq 生产者失败: %w", err)
	}
	return nil
}
