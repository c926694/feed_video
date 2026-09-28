package repo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const followKeyFormat = "follow:%d"

// Follow follow 表
type Follow struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Following uint64    `gorm:"not null;uniqueIndex:idx_user_follow" json:"user_id"`
	Follower  uint64    `gorm:"not null;uniqueIndex:idx_user_follow" json:"follow_user_id"`
	CreateTime time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)" json:"created_at"`
}

// Repo 关注关系的读写，集合在 Redis，关系表在 MySQL
type Repo struct {
	db          *gorm.DB
	redisClient *redis.Client
}

func New(db *gorm.DB, redisClient *redis.Client) *Repo {
	return &Repo{db: db, redisClient: redisClient}
}

var setScript = redis.NewScript(`
local want = tonumber(ARGV[2])
local cur = redis.call("SISMEMBER", KEYS[1], ARGV[1])
if cur == want then
    return 0
end
if want == 1 then
    redis.call("SADD", KEYS[1], ARGV[1])
else
    redis.call("SREM", KEYS[1], ARGV[1])
end
return 1
`)

// Set 把关注状态设置成目标态，返回状态是否发生了变化。
// 执行完之后集合状态必然等于目标态
func (r *Repo) Set(ctx context.Context, follower uint64, following uint64, active bool) (bool, error) {
	val := "0"
	if active {
		val = "1"
	}
	result, err := setScript.Run(ctx, r.redisClient, []string{followKey(follower)}, following, val).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// Create 写入一条关注关系，冲突时忽略，保证事件重放幂等
func (r *Repo) Create(ctx context.Context, follower uint64, following uint64) error {
	item := Follow{Following: following, Follower: follower}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error
}

// UserExists 判断用户是否存在，关注前校验目标
func (r *Repo) UserExists(ctx context.Context, userID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("user").Where("id = ?", userID).Count(&count).Error
	return count > 0, err
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

// CountFollowing 统计用户关注的人数
func (r *Repo) CountFollowing(ctx context.Context, follower uint64) (int64, error) {
	return r.redisClient.SCard(ctx, followKey(follower)).Result()
}

// CountFollowers 统计用户的粉丝数
func (r *Repo) CountFollowers(ctx context.Context, following uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Follow{}).Where("following = ?", following).Count(&count).Error
	return count, err
}
