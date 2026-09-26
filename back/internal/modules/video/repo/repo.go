package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	infoCacheKeyFormat = "cache:video:info:%d"
	infoLockKeyFormat  = "lock:video:info:%d"
	infoPhysicalTTL    = 24 * time.Hour
)

// Video video 表
type Video struct {
	ID           uint64    `gorm:"primaryKey"`
	AuthorID     uint64    `gorm:"index;not null;"`
	AuthorName   string    `gorm:"size:255;not null;"`
	AuthorAvatar string    `gorm:"size:255"`
	PlayURL      string    `gorm:"size:255;not null"`
	CoverURL     string    `gorm:"size:255;not null"`
	Title        string    `gorm:"size:255;not null"`
	Description  string    `gorm:"type:text"`
	LikeCount    int64     `gorm:"default:0"`
	CommentCount int64     `gorm:"default:0"`
	CreateTime   time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)" json:"created_at"`
	UpdateTime   time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP(3)" json:"updated_at"`
}

// InfoCacheEntry 视频信息缓存里的一条记录
type InfoCacheEntry struct {
	ID           uint64    `json:"id"`
	AuthorID     uint64    `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar string    `json:"author_avatar"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	CoverURL     string    `json:"cover_url"`
	PlayURL      string    `json:"play_url"`
	LikeCount    int64     `json:"like_count"`
	CommentCount int64     `json:"comment_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// InfoCache 带逻辑过期时间的缓存记录，Empty 表示这是一个空值缓存
type InfoCache struct {
	Entry    *InfoCacheEntry `json:"data,omitempty"`
	Empty    bool            `json:"empty"`
	ExpireAt int64           `json:"expire_at"`
}

// Repo video 表与视频信息缓存的读写
type Repo struct {
	db          *gorm.DB
	redisClient *redis.Client
}

func New(db *gorm.DB, redisClient *redis.Client) *Repo {
	return &Repo{db: db, redisClient: redisClient}
}

func (r *Repo) Create(ctx context.Context, item *Video) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repo) GetByID(ctx context.Context, videoID uint64) (*Video, error) {
	var item Video
	if err := r.db.WithContext(ctx).Where("id = ?", videoID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repo) FilterByIDs(ctx context.Context, videoIDs []uint64) ([]Video, error) {
	if len(videoIDs) == 0 {
		return []Video{}, nil
	}
	items := make([]Video, 0, len(videoIDs))
	if err := r.db.WithContext(ctx).Where("id in ?", videoIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repo) ListByAuthor(ctx context.Context, authorID uint64, limit uint64) ([]Video, error) {
	items := make([]Video, 0, limit)
	err := r.db.WithContext(ctx).
		Where("author_id = ?", authorID).
		Order("created_at desc").
		Limit(int(limit)).
		Find(&items).Error
	return items, err
}

func (r *Repo) CountByAuthor(ctx context.Context, authorID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Video{}).Where("author_id = ?", authorID).Count(&count).Error
	return count, err
}

func (r *Repo) ListByAuthorsBefore(ctx context.Context, authorIDs []uint64, limit uint64, before *time.Time) ([]Video, error) {
	if len(authorIDs) == 0 {
		return []Video{}, nil
	}
	items := make([]Video, 0, limit)
	query := r.db.WithContext(ctx).Model(&Video{}).Where("author_id in ?", authorIDs)
	if before != nil {
		query = query.Where("created_at < ?", *before)
	}
	err := query.Order("created_at desc").Limit(int(limit)).Find(&items).Error
	return items, err
}

func (r *Repo) Delete(ctx context.Context, videoID uint64) error {
	return r.db.WithContext(ctx).Delete(&Video{}, videoID).Error
}

func (r *Repo) IncreaseLikeCount(ctx context.Context, videoID uint64) error {
	return r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", videoID).
		Update("like_count", gorm.Expr("like_count + 1")).Error
}

func (r *Repo) DecreaseLikeCount(ctx context.Context, videoID uint64) error {
	return r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", videoID).
		Update("like_count", gorm.Expr("CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END")).Error
}

func (r *Repo) IncreaseCommentCount(ctx context.Context, videoID uint64) error {
	return r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", videoID).
		Update("comment_count", gorm.Expr("comment_count + 1")).Error
}

func (r *Repo) DecreaseCommentCount(ctx context.Context, videoID uint64) error {
	return r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", videoID).
		Update("comment_count", gorm.Expr("CASE WHEN comment_count > 0 THEN comment_count - 1 ELSE 0 END")).Error
}

// UpdateAuthorInfo 刷新某个作者全部视频上冗余的展示字段
func (r *Repo) UpdateAuthorInfo(ctx context.Context, authorID uint64, authorName string, authorAvatar string) error {
	updates := map[string]any{"author_avatar": authorAvatar}
	if authorName != "" {
		updates["author_name"] = authorName
	}
	return r.db.WithContext(ctx).Model(&Video{}).Where("author_id = ?", authorID).Updates(updates).Error
}

// GetInfoCache 读取视频信息缓存，返回 nil 表示缓存里没有
func (r *Repo) GetInfoCache(ctx context.Context, videoID uint64) (*InfoCache, error) {
	raw, err := r.redisClient.Get(ctx, infoCacheKey(videoID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cache InfoCache
	if err = json.Unmarshal([]byte(raw), &cache); err != nil {
		_ = r.redisClient.Del(ctx, infoCacheKey(videoID)).Err()
		return nil, nil
	}
	return &cache, nil
}

// SetInfoCache 写入视频信息缓存，实际有效期远长于记录里的逻辑过期时间
func (r *Repo) SetInfoCache(ctx context.Context, videoID uint64, cache InfoCache) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return r.redisClient.Set(ctx, infoCacheKey(videoID), data, infoPhysicalTTL).Err()
}

// DeleteInfoCache 让缓存失效
func (r *Repo) DeleteInfoCache(ctx context.Context, videoID uint64) error {
	return r.redisClient.Del(ctx, infoCacheKey(videoID)).Err()
}

// TryLockRebuild 尝试取到重建缓存的锁，返回的 token 用于解锁
func (r *Repo) TryLockRebuild(ctx context.Context, videoID uint64, ttl time.Duration) (string, bool, error) {
	token := strconv.FormatInt(time.Now().UnixNano(), 10)
	locked, err := r.redisClient.SetNX(ctx, infoLockKey(videoID), token, ttl).Result()
	if err != nil {
		return "", false, err
	}
	return token, locked, nil
}

// UnlockRebuild 释放重建缓存的锁，只有持有同一个 token 时才真正删除
func (r *Repo) UnlockRebuild(ctx context.Context, videoID uint64, token string) error {
	return unlockScript.Run(ctx, r.redisClient, []string{infoLockKey(videoID)}, token).Err()
}

var unlockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
`)

func infoCacheKey(videoID uint64) string {
	return fmt.Sprintf(infoCacheKeyFormat, videoID)
}

func infoLockKey(videoID uint64) string {
	return fmt.Sprintf(infoLockKeyFormat, videoID)
}
