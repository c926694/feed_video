package event

import "time"

// CreatedEvent 视频创建事件，feed 收到后把视频加进 Feed 索引
type CreatedEvent struct {
	VideoID   uint64    `json:"videoId"`
	AuthorID  uint64    `json:"authorId"`
	CreatedAt time.Time `json:"createdAt"`
}

// DeletedEvent 视频删除事件，video 用来删物理文件，comment 用来删评论，feed 用来清索引
type DeletedEvent struct {
	VideoID  uint64 `json:"videoId"`
	AuthorID uint64 `json:"authorId"`
	PlayURL  string `json:"playUrl"`
	CoverURL string `json:"coverUrl"`
}
