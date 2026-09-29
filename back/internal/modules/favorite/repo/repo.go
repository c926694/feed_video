package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const favoriteKeyFormat = "favorite:video:%d"

// Favorite user_favorite 表
type Favorite struct {
	ID         uint64    `gorm:"primaryKey"`
	UserID     uint64    `gorm:"column:user_id;not null"`
	VideoID    uint64    `gorm:"column:video_id;not null"`
	CreateTime time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)" json:"created_at"`
}

// TableName 返回 user_favorite
func (Favorite) TableName() string {
	return "user_favorite"
}

// Repo 收藏数据的读写。集合在 Redis，关系表在 MySQL
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

// Set 把收藏状态设置成目标态，返回状态是否发生了变化。
// 执行完之后集合状态必然等于目标态
func (r *Repo) Set(ctx context.Context, videoID uint64, userID uint64, active bool) (bool, error) {
	val := "0"
	if active {
		val = "1"
	}
	result, err := setScript.Run(ctx, r.redisClient, []string{favoriteKey(videoID)}, userID, val).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// FilterFavorited 批量查询用户是否收藏了这些视频
func (r *Repo) FilterFavorited(ctx context.Context, userID uint64, videoIDs []uint64) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(videoIDs))
	if len(videoIDs) == 0 {
		return result, nil
	}
	pipeline := r.redisClient.Pipeline()
	commands := make([]*redis.BoolCmd, len(videoIDs))
	for i, videoID := range videoIDs {
		commands[i] = pipeline.SIsMember(ctx, favoriteKey(videoID), userID)
	}
	if _, err := pipeline.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	for i, videoID := range videoIDs {
		favorited, err := commands[i].Result()
		if err != nil && err != redis.Nil {
			return nil, err
		}
		result[videoID] = favorited
	}
	return result, nil
}

func favoriteKey(videoID uint64) string {
	return fmt.Sprintf(favoriteKeyFormat, videoID)
}

// Create 写入一条收藏关系，冲突时忽略，保证事件重放幂等
func (r *Repo) Create(ctx context.Context, userID uint64, videoID uint64) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&Favorite{UserID: userID, VideoID: videoID}).Error
}

// Delete 删除一条收藏关系，本身幂等
func (r *Repo) Delete(ctx context.Context, userID uint64, videoID uint64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND video_id = ?", userID, videoID).
		Delete(&Favorite{}).Error
}

// DeleteByVideo 删除某个视频下的全部收藏，视频删除时清理
func (r *Repo) DeleteByVideo(ctx context.Context, videoID uint64) error {
	return r.db.WithContext(ctx).
		Where("video_id = ?", videoID).
		Delete(&Favorite{}).Error
}

// DeleteTargetSet 删除某个视频的收藏集合，视频删除时清理
func (r *Repo) DeleteTargetSet(ctx context.Context, videoID uint64) error {
	return r.redisClient.Del(ctx, favoriteKey(videoID)).Err()
}

// CountFavorites 返回某个视频的收藏用户数
func (r *Repo) CountFavorites(ctx context.Context, videoID uint64) (int64, error) {
	return r.redisClient.SCard(ctx, favoriteKey(videoID)).Result()
}

// ListByUser 收藏列表：按收藏时间倒序双字段游标分页，联视频表取展示字段
func (r *Repo) ListByUser(ctx context.Context, userID uint64, lastCreatedAt time.Time, lastID uint64, limit int) ([]Video, error) {
	items := make([]Video, 0, limit)
	query := r.db.WithContext(ctx).
		Table("user_favorite AS f").
		Select("v.id, v.author_id, v.author_name, v.author_avatar, v.title, v.description, v.cover_url, v.play_url, v.like_count, v.comment_count, v.favorite_count, v.status, v.created_at, f.created_at AS favorited_at, f.id AS favorite_id").
		Joins("JOIN video AS v ON v.id = f.video_id").
		Where("f.user_id = ? AND v.status = ?", userID, "published")
	if lastID > 0 {
		query = query.Where("f.created_at <= ? AND f.id < ?", lastCreatedAt, lastID)
	}
	if err := query.Order("f.created_at desc, f.id desc").Limit(limit).Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Video 收藏列表联表查询出的视频行
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
	FavoritedAt   time.Time `gorm:"column:favorited_at"`
	FavoriteID    uint64    `gorm:"column:favorite_id"`
}
