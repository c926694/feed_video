package event

type LikeVideoEvent struct {
	EventType string `json:"event"`
	VideoId   uint64 `json:"videoId"`
}

type LikeCommentEvent struct {
	EventType string `json:"event"`
	CommentId uint64 `json:"commentId"`
}

type DeleteVideoEvent struct {
	PlayURL  string `json:"playUrl"`
	CoverURL string `json:"coverUrl"`
}

type FollowEvent struct {
	EventType string `json:"event"`
	Following uint64 `json:"following"`
	Follower  uint64 `json:"follower"`
}

type VideoHotEvent struct {
	VideoId     uint64  `json:"videoId"`
	ScoreDelta  float64 `json:"scoreDelta"`
	MinuteStamp int64   `json:"minuteStamp,omitempty"`
}

const (
	Like     = "like"
	Dislike  = "dislike"
	Follow   = "follow"
	Unfollow = "unfollow"
)
