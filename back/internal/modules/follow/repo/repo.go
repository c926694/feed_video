package repo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const followKeyFormat = "follow:%d"

// Follow follow 表
type Follow struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Following uint64    `gorm:"not null;uniqueIndex:idx_user_follow" json:"user_id"`
	Follower  uint64    `gorm:"not null;uniqueIndex:idx_user_follow" json:"follow_user_id"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;not null"`
}

// Repo 关注关系的读写，集合在 Redis，关系表在 MySQL
type Repo struct {
	db          *gorm.DB
	redisClient *redis.Client
}

func New(db *gorm.DB, redisClient *redis.Client) *Repo {
	return &Repo{db: db, redisClient: redisClient}
}

var switchScript = redis.NewScript(`
if redis.call("SISMEMBER", KEYS[1], ARGV[1]) == 1 then
    redis.call("SREM", KEYS[1], ARGV[1])
    return 0
end
redis.call("SADD", KEYS[1], ARGV[1])
return 1
`)

// Switch 切换关注状态，返回切换后的状态
func (r *Repo) Switch(ctx context.Context, follower uint64, following uint64) (bool, error) {
	result, err := switchScript.Run(ctx, r.redisClient, []string{followKey(follower)}, following).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// Reset 把状态恢复成切换之前的样子，用于事件发布失败时回滚
func (r *Repo) Reset(ctx context.Context, follower uint64, following uint64, followed bool) error {
	key := followKey(follower)
	if followed {
		return r.redisClient.SAdd(ctx, key, following).Err()
	}
	return r.redisClient.SRem(ctx, key, following).Err()
}

func (r *Repo) Create(ctx context.Context, follower uint64, following uint64) error {
	item := Follow{Following: following, Follower: follower}
	return r.db.WithContext(ctx).Create(&item).Error
}

func (r *Repo) Delete(ctx context.Context, follower uint64, following uint64) error {
	return r.db.WithContext(ctx).
		Where("follower = ? and following = ?", follower, following).
		Delete(&Follow{}).Error
}

// FilterFollowing 批量查询用户是否关注了这些人
func (r *Repo) FilterFollowing(ctx context.Context, follower uint64, followingIDs []uint64) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(followingIDs))
	if len(followingIDs) == 0 {
		return result, nil
	}
	key := followKey(follower)
	pipeline := r.redisClient.Pipeline()
	commands := make([]*redis.BoolCmd, len(followingIDs))
	for i, followingID := range followingIDs {
		commands[i] = pipeline.SIsMember(ctx, key, followingID)
	}
	if _, err := pipeline.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	for i, followingID := range followingIDs {
		followed, err := commands[i].Result()
		if err != nil && err != redis.Nil {
			return nil, err
		}
		result[followingID] = followed
	}
	return result, nil
}

// FollowingIDs 取用户关注的全部人
func (r *Repo) FollowingIDs(ctx context.Context, follower uint64) ([]uint64, error) {
	members, err := r.redisClient.SMembers(ctx, followKey(follower)).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(members))
	for _, member := range members {
		id, parseErr := strconv.ParseUint(member, 10, 64)
		if parseErr != nil {
			return nil, parseErr
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func followKey(follower uint64) string {
	return fmt.Sprintf(followKeyFormat, follower)
}
