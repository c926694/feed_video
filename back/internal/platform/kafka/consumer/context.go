package consumer

import "context"

// eventIDKey 事件 ID 在 context 里的键
type eventIDKey struct{}

// WithEventID 把当前消息的事件 ID 放进 context，供处理函数写日志或做幂等判断
func WithEventID(ctx context.Context, eventID string) context.Context {
	if eventID == "" {
		return ctx
	}
	return context.WithValue(ctx, eventIDKey{}, eventID)
}

// EventIDFromContext 取当前消息的事件 ID，取不到返回空串
func EventIDFromContext(ctx context.Context) string {
	eventID, _ := ctx.Value(eventIDKey{}).(string)
	return eventID
}
