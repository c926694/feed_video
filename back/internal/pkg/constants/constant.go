package constants

import "os"

const (
	DefaultAvatar            = "avatar/default.svg"
	AvatarPrefix             = "/avatar/"
	CoverPrefix              = "/cover/"
	VideoPrefix              = "/video/"
	FeedVideoKey             = "feed:video"
	HotFeedVideoKey          = "feed:hot:video"
	HotFeedVideoMinutePrefix = "feed:hot:video:1m"
	HotFeedVideoMergePrefix  = "feed:hot:video:merge"
	VideoInfoCacheKey        = "cache:video:info:%d"
	VideoInfoLockKey         = "lock:video:info:%d"
	LikeVideo                = "like:video:%d"
	LikeComment              = "like:comment:%d"
	FollowKey                = "follow:%d"
)

var (
	StoragePath = getEnv("STORAGE_PATH", "D:/develop/nginx-1.22.0-web/storage")
	HttpPath    = getEnv("HTTP_PATH", "/static/")
)

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
