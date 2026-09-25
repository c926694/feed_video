package user

import "mime/multipart"

// LoginReq 登录请求
type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterReq 注册请求
type RegisterReq struct {
	LoginReq
	RePassword string `json:"re_password"`
}

// UpdateProfileReq 修改资料请求
type UpdateProfileReq struct {
	Nickname string                `form:"nickname"`
	Avatar   *multipart.FileHeader `form:"avatar"`
}

// InfoRes 用户信息响应
type InfoRes struct {
	UserID        uint64 `json:"user_id"`
	Username      string `json:"username"`
	Nickname      string `json:"nickname"`
	AvatarURL     string `json:"avatar_URL"`
	FollowCount   int64  `json:"follow_count"`
	FollowerCount int64  `json:"follower_count"`
	VideoCount    int64  `json:"video_count"`
}
