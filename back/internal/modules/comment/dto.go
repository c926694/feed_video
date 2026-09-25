package comment

import "time"

// CreateReq 发表评论请求
type CreateReq struct {
	VideoID uint64 `json:"video_id"`
	Content string `json:"content"`
}

// AuthorRes 评论作者信息
type AuthorRes struct {
	UserID    uint64 `json:"user_id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_URL"`
}

// InfoRes 评论响应
type InfoRes struct {
	Id        uint64    `json:"id"`
	VideoId   uint64    `json:"video_id"`
	Commenter uint64    `json:"commenter"`
	Content   string    `json:"content"`
	LikeCount int64     `json:"like_count"`
	IsLiked   bool      `json:"is_liked"`
	Author    AuthorRes `json:"author"`
	CreatedAt time.Time `json:"created_at"`

	// 内部字段，与评论行上的冗余列对应，不参与 JSON 输出
	commenterName   string
	commenterAvatar string
}
