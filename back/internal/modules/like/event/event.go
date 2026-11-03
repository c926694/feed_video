package event

// 点赞目标的类型
const (
	TargetVideo   = "video"
	TargetComment = "comment"
)

// SwitchedEvent 点赞状态切换事件。video、comment、feed 各自订阅，只处理自己关心的目标类型。
// OccurredAt 是这次切换的发生时间（Unix 毫秒），热度按它决定计入哪一分钟
type SwitchedEvent struct {
	Target     string `json:"target"`
	TargetID   uint64 `json:"targetId"`
	Liked      bool   `json:"liked"`
	Operator   uint64 `json:"operator"`
	OccurredAt int64  `json:"occurredAt"`
}
