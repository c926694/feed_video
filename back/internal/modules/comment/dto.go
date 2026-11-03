package comment

import "time"

// CreateReq 发表评论请求，parent_id 为 0 表示顶级评论
type CreateReq struct {
	VideoID   uint64 `json:"video_id"`
	Content   string `json:"content"`
	ParentID  uint64 `json:"parent_id"`
	ReplyToID uint64 `json:"reply_to_id"`
}

// AuthorRes 评论作者信息
type AuthorRes struct {
	UserID    uint64 `json:"user_id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_URL"`
}

// InfoRes 评论响应，子评论走回复分页接口按需加载
type InfoRes struct {
	Id              uint64    `json:"id"`
	VideoId         uint64    `json:"video_id"`
	ParentId        uint64    `json:"parent_id"`
	ReplyToId       uint64    `json:"reply_to_id"`
	ReplyToUserID   uint64    `json:"reply_to_user_id"`
	ReplyToUserName string    `json:"reply_to_user_name"`
	Commenter       uint64    `json:"commenter"`
	Content         string    `json:"content"`
	LikeCount       int64     `json:"like_count"`
	ReplyCount      int64     `json:"reply_count"`
	IsLiked         bool      `json:"is_liked"`
	Author          AuthorRes `json:"author"`
	CreatedAt       time.Time `json:"created_at"`

	// 内部字段，与评论行上的冗余列对应，不参与 JSON 输出
	commenterName   string
	commenterAvatar string
}

// ListRes 评论列表分页响应
type ListRes struct {
	List          []InfoRes `json:"list"`
	LastCreatedAt int64     `json:"last_created_at"`
	LastID        uint64    `json:"last_id"`
	HasMore       bool      `json:"has_more"`
}
