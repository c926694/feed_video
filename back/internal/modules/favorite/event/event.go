package event

// SwitchedEvent 收藏状态切换事件。
// video 订阅后按集合大小对账收藏数，feed 订阅后加热度，favorite 订阅后维护关系表。
// OccurredAt 是这次切换的发生时间（Unix 毫秒），热度按它决定计入哪一分钟
type SwitchedEvent struct {
	VideoID    uint64 `json:"videoId"`
	UserID     uint64 `json:"userId"`
	Favorited  bool   `json:"favorited"`
	OccurredAt int64  `json:"occurredAt"`
}
