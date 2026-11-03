package follow

// FollowRes 关注状态响应
type FollowRes struct {
	Following uint64 `json:"following"`
	IsFollow  bool   `json:"is_follow"`
}

// FollowUserItem 关注或粉丝列表里的一项
type FollowUserItem struct {
	UserID    uint64 `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_URL"`
	// IsFollow 当前登录用户是否关注了列表里的这个人
	IsFollow bool `json:"is_follow"`
}

// FollowListRes 关注列表与粉丝列表的响应
type FollowListRes struct {
	List    []FollowUserItem `json:"list"`
	LastId  uint64           `json:"last_id"`
	HasMore bool             `json:"has_more"`
}
