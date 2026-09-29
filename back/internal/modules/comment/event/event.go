package event

// CreatedEvent 评论创建事件，feed 订阅后加热度。
// 评论数由写路径的事务同步维护，评论删除也不回减热度，所以没有删除事件。
// OccurredAt 是评论的创建时间（Unix 毫秒），热度按它决定计入哪一分钟
type CreatedEvent struct {
	CommentID  uint64 `json:"commentId"`
	VideoID    uint64 `json:"videoId"`
	Commenter  uint64 `json:"commenter"`
	OccurredAt int64  `json:"occurredAt"`
}
