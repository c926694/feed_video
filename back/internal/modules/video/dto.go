package video

import (
	"mime/multipart"
	"time"
)

// CreateReq 发布视频请求
type CreateReq struct {
	Title       string                `form:"title"`
	Description string                `form:"description"`
	Cover       *multipart.FileHeader `form:"cover"`
	Play        *multipart.FileHeader `form:"play"`
}

// CreateRes 发布视频响应
type CreateRes struct {
	Id  uint64 `json:"id"`
	Url string `json:"url"`
}

// InfoRes 视频信息响应
type InfoRes struct {
	Id           uint64    `json:"id"`
	AuthorID     uint64    `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar string    `json:"author_avatar"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	CoverURL     string    `json:"coverURL"`
	PlayURL      string    `json:"playURL"`
	CommentCount int64     `json:"comment_count"`
	LikeCount    int64     `json:"like_count"`
	IsLiked      bool      `json:"is_liked"`
	IsFollow     bool      `json:"is_follow"`
	CreatedAt    time.Time `json:"created_at"`
}
