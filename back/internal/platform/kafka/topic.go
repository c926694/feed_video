package kafka

// 全部 topic 名称的唯一来源
const (
	TopicLikeVideo   = "like_video"
	TopicLikeComment = "like_comment"
	TopicFollow      = "follow"
	TopicVideoHot    = "video_hot"
	TopicVideoDelete = "video_delete"
)

// FailedSuffix 处理失败的消息转发到 "<topic>.failed"
const FailedSuffix = ".failed"

// allTopics 启动时需要确保存在的 topic
var allTopics = []string{
	TopicLikeVideo,
	TopicLikeComment,
	TopicFollow,
	TopicVideoHot,
	TopicVideoDelete,
}
