package follow

// FollowRes 关注状态响应
type FollowRes struct {
	Following uint64 `json:"following"`
	IsFollow  bool   `json:"is_follow"`
}
