package event

// UpdatedEvent 用户资料变更事件。video 与 comment 订阅后刷新自己表里冗余的作者展示字段。
type UpdatedEvent struct {
	UserID    uint64 `json:"userId"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl"`
}
