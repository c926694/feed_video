package like

// VideoLikeRes 视频点赞响应
type VideoLikeRes struct {
	VideoId uint64 `json:"video_id"`
	IsLiked bool   `json:"is_liked"`
}

// CommentLikeRes 评论点赞响应
type CommentLikeRes struct {
	CommentId uint64 `json:"comment_id"`
	IsLiked   bool   `json:"is_liked"`
}
