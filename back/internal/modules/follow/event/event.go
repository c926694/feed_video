package event

// SwitchedEvent 关注状态切换事件，user 订阅后更新双方的关注计数，follow 订阅后维护关系表
type SwitchedEvent struct {
	Follower  uint64 `json:"follower"`
	Following uint64 `json:"following"`
	Followed  bool   `json:"followed"`
}
