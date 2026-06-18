package message

type LikeVideoMessage struct {
	EventType string `json:"event"`
	VideoId   uint64 `json:"videoId"`
}

type LikeCommentMessage struct {
	EventType string `json:"event"`
	CommentId uint64 `json:"commentId"`
}

type DeleteVideoMessage struct {
	PlayURL  string `json:"playUrl"`
	CoverURL string `json:"coverUrl"`
}

type FollowMessage struct {
	EventType string `json:"event"`
	Following uint64 `json:"following"`
	Follower  uint64 `json:"follower"`
}

type VideoHotMessage struct {
	VideoId     uint64  `json:"videoId"`
	ScoreDelta  float64 `json:"scoreDelta"`
	MinuteStamp int64   `json:"minuteStamp,omitempty"`
}
