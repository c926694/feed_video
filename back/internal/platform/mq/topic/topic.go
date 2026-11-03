package topic

// 全部 topic 名称的唯一来源
const (
	VideoCreated         = "video_created"
	VideoDeleted         = "video_deleted"
	VideoPrivateSwitched = "video_private_switched"
	CommentCreated       = "comment_created"
	CommentDeleted       = "comment_deleted"
	LikeSwitched         = "like_switched"
	FavoriteSwitched     = "favorite_switched"
	FollowSwitched       = "follow_switched"
	UserUpdated          = "user_updated"
)

// FailedSuffix 处理失败的消息转发到 "<topic>_failed"。
// RocketMQ 的 topic 名只允许字母数字与下划线，所以这里用下划线。
const FailedSuffix = "_failed"

// FailedTopic 返回某个业务 topic 对应的失败留档 topic
func FailedTopic(name string) string {
	return name + FailedSuffix
}

// All 返回全部业务 topic，供启动时预登记与初始化脚本核对
func All() []string {
	return []string{
		VideoCreated,
		VideoDeleted,
		VideoPrivateSwitched,
		CommentCreated,
		CommentDeleted,
		LikeSwitched,
		FavoriteSwitched,
		FollowSwitched,
		UserUpdated,
	}
}
