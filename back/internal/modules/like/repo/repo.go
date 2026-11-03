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

// Set 把点赞状态设置成目标态，返回状态是否发生了变化。
// 执行完之后集合状态必然等于目标态
func (r *Repo) Set(ctx context.Context, target string, targetID uint64, userID uint64, active bool) (bool, error) {
	val := "0"
	if active {
		val = "1"
	}
	result, err := setScript.Run(ctx, r.redisClient, []string{keyFor(target, targetID)}, userID, val).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// AppendLiked 把这一批目标的点赞状态查询追加到给定管道，供调用方合并成一次往返
func (r *Repo) AppendLiked(pipe redis.Pipeliner, ctx context.Context, target string, userID uint64, targetIDs []uint64) []*redis.BoolCmd {
	commands := make([]*redis.BoolCmd, len(targetIDs))
	for i, targetID := range targetIDs {
		commands[i] = pipe.SIsMember(ctx, keyFor(target, targetID), userID)
	}
	return commands
}

// FilterLiked 批量查询用户是否点赞了这些目标
func (r *Repo) FilterLiked(ctx context.Context, target string, userID uint64, targetIDs []uint64) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(targetIDs))
	if len(targetIDs) == 0 {
		return result, nil
	}
	pipeline := r.redisClient.Pipeline()
	commands := r.AppendLiked(pipeline, ctx, target, userID, targetIDs)
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

// DeleteByTargets 批量删除多个目标下的全部点赞关系，一条 SQL
func (r *Repo) DeleteByTargets(ctx context.Context, targetType string, targetIDs []uint64) error {
	if len(targetIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("target_type = ? AND target_id IN ?", targetType, targetIDs).
		Delete(&Like{}).Error
}

// DeleteTargetSet 删除某个目标的点赞集合，删除视频或评论时清理
func (r *Repo) DeleteTargetSet(ctx context.Context, target string, targetID uint64) error {
	return r.redisClient.Del(ctx, keyFor(target, targetID)).Err()
}

// DeleteTargetSets 批量删除多个目标的点赞集合，一条 DEL 命令
func (r *Repo) DeleteTargetSets(ctx context.Context, target string, targetIDs []uint64) error {
	if len(targetIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		keys = append(keys, keyFor(target, targetID))
	}
	return r.redisClient.Del(ctx, keys...).Err()
}

// CountLikes 返回某个目标的点赞用户数
func (r *Repo) CountLikes(ctx context.Context, target string, targetID uint64) (int64, error) {
	return r.redisClient.SCard(ctx, keyFor(target, targetID)).Result()
}

// ListVideosByUser 我的点赞列表：按点赞时间倒序双字段游标分页，联视频表取展示字段
func (r *Repo) ListVideosByUser(ctx context.Context, userID uint64, lastCreatedAt time.Time, lastID uint64, limit int) ([]Video, error) {
	items := make([]Video, 0, limit)
	query := r.db.WithContext(ctx).
		Table("user_like AS ul").
		Select("v.id, v.author_id, v.author_name, v.author_avatar, v.title, v.description, v.cover_url, v.play_url, v.like_count, v.comment_count, v.favorite_count, v.status, v.created_at, ul.created_at AS liked_at, ul.id AS like_id").
		Joins("JOIN video AS v ON v.id = ul.target_id").
		Where("ul.user_id = ? AND ul.target_type = ? AND v.status = ?", userID, "video", "published")
	if lastID > 0 {
		query = query.Where("ul.created_at <= ? AND ul.id < ?", lastCreatedAt, lastID)
	}
	if err := query.Order("ul.created_at desc, ul.id desc").Limit(limit).Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Video 点赞列表联表查询出的视频行
type Video struct {
	ID            uint64    `gorm:"column:id"`
	AuthorID      uint64    `gorm:"column:author_id"`
	AuthorName    string    `gorm:"column:author_name"`
	AuthorAvatar  string    `gorm:"column:author_avatar"`
	Title         string    `gorm:"column:title"`
	Description   string    `gorm:"column:description"`
	CoverURL      string    `gorm:"column:cover_url"`
	PlayURL       string    `gorm:"column:play_url"`
	LikeCount     int64     `gorm:"column:like_count"`
	CommentCount  int64     `gorm:"column:comment_count"`
	FavoriteCount int64     `gorm:"column:favorite_count"`
	Status        string    `gorm:"column:status"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	LikedAt       time.Time `gorm:"column:liked_at"`
	LikeID        uint64    `gorm:"column:like_id"`
}
