package message

import "time"

// ActorItem 通知的触发者，只带列表要展示的字段
type ActorItem struct {
	ID        uint64 `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_URL"`
}

// MessageItem 通知列表里的一条会话
type MessageItem struct {
	ID         uint64      `json:"id"`
	Type       string      `json:"type"`
	Title      string      `json:"title"`
	Content    string      `json:"content"`
	Actors     []ActorItem `json:"actors"`
	ActorCount uint32      `json:"actor_count"`
	VideoID    uint64      `json:"video_id"`
	CommentID  uint64      `json:"comment_id"`
	CoverURL   string      `json:"coverURL"`
	IsRead     bool        `json:"is_read"`
	CreatedAt  time.Time   `json:"created_at"`
}

// ListRes 通知列表响应
type ListRes struct {
	MessageList   []MessageItem `json:"message_list"`
	LastCreatedAt int64         `json:"last_created_at"`
	LastId        uint64        `json:"last_id"`
}

// UnreadRes 未读数响应
type UnreadRes struct {
	UnreadCount int64 `json:"unread_count"`
}

// ReadReq 标记已读请求，message_ids 为空表示全部已读
type ReadReq struct {
	MessageIDs []uint64 `json:"message_ids"`
}

// ReadRes 标记已读响应
type ReadRes struct {
	Updated int64 `json:"updated"`
}
