package repo

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	videoLikeKey   = "like:video:%d"
	commentLikeKey = "like:comment:%d"
)

// Repo 点赞数据的读写，点赞状态只存在 Redis 集合里
type Repo struct {
	redisClient *redis.Client
}

func New(redisClient *redis.Client) *Repo {
	return &Repo{redisClient: redisClient}
}

var switchScript = redis.NewScript(`
if redis.call("SISMEMBER", KEYS[1], ARGV[1]) == 1 then
    redis.call("SREM", KEYS[1], ARGV[1])
    return 0
end
redis.call("SADD", KEYS[1], ARGV[1])
return 1
`)

// Switch 切换某个目标上的点赞状态，返回切换后的状态
func (r *Repo) Switch(ctx context.Context, target string, targetID uint64, userID uint64) (bool, error) {
	key := keyFor(target, targetID)
	result, err := switchScript.Run(ctx, r.redisClient, []string{key}, userID).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// Reset 把状态恢复成切换之前的样子，用于事件发布失败时回滚
func (r *Repo) Reset(ctx context.Context, target string, targetID uint64, userID uint64, liked bool) error {
	key := keyFor(target, targetID)
	if liked {
		return r.redisClient.SAdd(ctx, key, userID).Err()
	}
	return r.redisClient.SRem(ctx, key, userID).Err()
}

// FilterLiked 批量查询用户是否点赞了这些目标
func (r *Repo) FilterLiked(ctx context.Context, target string, userID uint64, targetIDs []uint64) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(targetIDs))
	if len(targetIDs) == 0 {
		return result, nil
	}
	pipeline := r.redisClient.Pipeline()
	commands := make([]*redis.BoolCmd, len(targetIDs))
	for i, targetID := range targetIDs {
		commands[i] = pipeline.SIsMember(ctx, keyFor(target, targetID), userID)
	}
	if _, err := pipeline.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	for i, targetID := range targetIDs {
		liked, err := commands[i].Result()
		if err != nil && err != redis.Nil {
			return nil, err
		}
		result[targetID] = liked
	}
	return result, nil
}

func keyFor(target string, targetID uint64) string {
	if target == "comment" {
		return fmt.Sprintf(commentLikeKey, targetID)
	}
	return fmt.Sprintf(videoLikeKey, targetID)
}
