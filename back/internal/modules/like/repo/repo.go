package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	videoLikeKey   = "like:video:%d"
	commentLikeKey = "like:comment:%d"
)

// Repo 点赞数据的读写。集合在 Redis，关系表在 MySQL。
type Repo struct {
	db          *gorm.DB
	redisClient *redis.Client
}

func New(db *gorm.DB, redisClient *redis.Client) *Repo {
	return &Repo{db: db, redisClient: redisClient}
}

// Like user_like 表，target_type 取值 video / comment
type Like struct {
	ID         uint64    `gorm:"primaryKey"`
	UserID     uint64    `gorm:"column:user_id;not null"`
	TargetType string    `gorm:"column:target_type;size:16;not null"`
	TargetID   uint64    `gorm:"column:target_id;not null"`
	CreateTime time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)" json:"created_at"`
}

// TableName 返回 user_like，避免使用 SQL 保留字 like
func (Like) TableName() string {
	return "user_like"
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

// Create 写入一条点赞关系，冲突时忽略，保证事件重放幂等
func (r *Repo) Create(ctx context.Context, userID uint64, targetType string, targetID uint64) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&Like{UserID: userID, TargetType: targetType, TargetID: targetID}).Error
}

// Delete 删除一条点赞关系，本身幂等
func (r *Repo) Delete(ctx context.Context, userID uint64, targetType string, targetID uint64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Delete(&Like{}).Error
}

// DeleteByTarget 删除某个目标下的全部点赞，删除视频或评论时清理
func (r *Repo) DeleteByTarget(ctx context.Context, targetType string, targetID uint64) error {
	return r.db.WithContext(ctx).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Delete(&Like{}).Error
}

// DeleteTargetSet 删除某个目标的点赞集合，删除视频或评论时清理
func (r *Repo) DeleteTargetSet(ctx context.Context, target string, targetID uint64) error {
	return r.redisClient.Del(ctx, keyFor(target, targetID)).Err()
}

// CountLikes 返回某个目标的点赞用户数
func (r *Repo) CountLikes(ctx context.Context, target string, targetID uint64) (int64, error) {
	return r.redisClient.SCard(ctx, keyFor(target, targetID)).Result()
}
