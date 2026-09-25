package topic

// 全部 topic 名称的唯一来源
const (
	VideoCreated   = "video_created"
	VideoDeleted   = "video_deleted"
	CommentCreated = "comment_created"
	CommentDeleted = "comment_deleted"
	LikeSwitched   = "like_switched"
	FollowSwitched = "follow_switched"
	UserUpdated    = "user_updated"
)

// FailedSuffix 处理失败的消息转发到 "<topic>.failed"
const FailedSuffix = ".failed"
