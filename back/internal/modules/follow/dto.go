package follow

// SwitchRes 关注切换响应
type SwitchRes struct {
	Following uint64 `json:"following"`
	IsFollow  bool   `json:"is_follow"`
}
