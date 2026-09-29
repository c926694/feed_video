package favorite

import "time"

// FavoriteRes 收藏状态响应
type FavoriteRes struct {
	VideoId     uint64 `json:"video_id"`
	IsFavorited bool   `json:"is_favorited"`
}

// ItemRes 收藏列表里的一条视频，字段与 Feed 条目一致
type ItemRes struct {
	Id            uint64    `json:"id"`
	AuthorID      uint64    `json:"author_id"`
	AuthorName    string    `json:"author_name"`
	AuthorAvatar  string    `json:"author_avatar"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	CoverURL      string    `json:"coverURL"`
	PlayURL       string    `json:"playURL"`
	CommentCount  int64     `json:"comment_count"`
	LikeCount     int64     `json:"like_count"`
	FavoriteCount int64     `json:"favorite_count"`
	IsLiked       bool      `json:"is_liked"`
	IsFavorited   bool      `json:"is_favorited"`
	IsFollow      bool      `json:"is_follow"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListRes 收藏列表分页响应
type ListRes struct {
	List          []ItemRes `json:"list"`
	LastCreatedAt int64     `json:"last_created_at"`
	LastID        uint64    `json:"last_id"`
	HasMore       bool      `json:"has_more"`
}
