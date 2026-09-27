package video

import "time"

// CreateReq 创建发布记录请求，文件由前端直传 OSS
type CreateReq struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	CoverKey    string `json:"cover_key"`
	PlayKey     string `json:"play_key"`
	RequestId   string `json:"request_id"`
}

// CreateRes 创建发布记录响应
type CreateRes struct {
	Id     uint64 `json:"id"`
	Status string `json:"status"`
}

// UpdateStatusReq 更新发布状态请求
type UpdateStatusReq struct {
	Status string `json:"status"`
}

// CredentialReq 上传凭证请求
type CredentialReq struct {
	CoverExt string `json:"cover_ext"`
	PlayExt  string `json:"play_ext"`
}

// CredentialRes 上传凭证响应
type CredentialRes struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SecurityToken   string `json:"security_token"`
	Expiration      int64  `json:"expiration"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	CoverKey        string `json:"cover_key"`
	PlayKey         string `json:"play_key"`
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
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
