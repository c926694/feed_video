package feed

import "time"

// VideoItem Feed 里的一条视频
type VideoItem struct {
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

// FeedRes 普通 Feed 响应
type FeedRes struct {
	FeedVideoList []VideoItem `json:"feed_video_list"`
	LastScore     float64     `json:"last_score"`
}

// HotFeedRes 热榜响应
type HotFeedRes struct {
	FeedVideoList []VideoItem `json:"feed_video_list"`
	NextOffset    uint64      `json:"next_offset"`
	HasMore       bool        `json:"has_more"`
	Interval      int         `json:"interval"`
}
