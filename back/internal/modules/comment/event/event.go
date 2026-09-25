package event

// CreatedEvent 评论创建事件，video 用来加评论数，feed 用来加热度
type CreatedEvent struct {
	CommentID uint64 `json:"commentId"`
	VideoID   uint64 `json:"videoId"`
	Commenter uint64 `json:"commenter"`
}

// DeletedEvent 评论删除事件，video 用来减评论数，feed 用来减热度
type DeletedEvent struct {
	CommentID uint64 `json:"commentId"`
	VideoID   uint64 `json:"videoId"`
}
