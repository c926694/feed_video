package eventid

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// Header 事件 ID 在 Kafka 消息头里的名字
const Header = "event_id"

// New 生成一个事件 ID。前 48 位是毫秒时间戳，后面是随机位（UUIDv7 的形状），
// 既能当唯一标识，也能在日志里按时间排序
func New() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(fmt.Sprintf("生成事件 ID 失败: %v", err))
	}

	millis := uint64(time.Now().UnixMilli())
	raw[0] = byte(millis >> 40)
	raw[1] = byte(millis >> 32)
	raw[2] = byte(millis >> 24)
	raw[3] = byte(millis >> 16)
	raw[4] = byte(millis >> 8)
	raw[5] = byte(millis)
	raw[6] = (raw[6] & 0x0f) | 0x70 // 版本 7
	raw[8] = (raw[8] & 0x3f) | 0x80 // 变体

	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

// FromHeaders 从消息头里取事件 ID，取不到返回空串
func FromHeaders(headers []kafka.Header) string {
	for _, header := range headers {
		if header.Key == Header {
			return string(header.Value)
		}
	}
	return ""
}

// HeaderOf 组装带事件 ID 的消息头
func HeaderOf(id string) []kafka.Header {
	return []kafka.Header{{Key: Header, Value: []byte(id)}}
}
