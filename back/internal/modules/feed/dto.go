package feed

import "time"

// VideoItem Feed 里的一条视频
type VideoItem struct {
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
	// Score 只在热榜里有值，是窗口内计得的加权互动分
	Score float64 `json:"score"`
}

// FeedRes 普通 Feed 响应
type FeedRes struct {
	FeedVideoList []VideoItem `json:"feed_video_list"`
	LastCreatedAt int64       `json:"last_created_at"`
	LastId        uint64      `json:"last_id"`
}

// HotFeedRes 热榜响应
type HotFeedRes struct {
	FeedVideoList []VideoItem `json:"feed_video_list"`
	NextOffset    uint64      `json:"next_offset"`
	HasMore       bool        `json:"has_more"`
	Interval      int         `json:"interval"`
}
