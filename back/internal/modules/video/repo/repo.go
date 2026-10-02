package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	infoCacheKeyFormat = "cache:video:info:%d"
	infoPhysicalTTL    = 24 * time.Hour
	infoMissTTL        = 2 * time.Minute
)

// 视频发布状态
const (
	StatusCreated   = "created"   // 已创建，文件上传中
	StatusPublished = "published" // 已发布
	StatusFailed    = "failed"    // 上传失败
)

// Video video 表
type Video struct {
	ID            uint64    `gorm:"primaryKey"`
	AuthorID      uint64    `gorm:"index;not null;"`
	AuthorName    string    `gorm:"size:255;not null;"`
	AuthorAvatar  string    `gorm:"size:255"`
	PlayURL       string    `gorm:"size:255;not null"`
	CoverURL      string    `gorm:"size:255;not null"`
	Title         string    `gorm:"size:255;not null"`
	Description   string    `gorm:"type:text"`
	LikeCount     int64     `gorm:"default:0"`
	CommentCount  int64     `gorm:"default:0"`
	FavoriteCount int64     `gorm:"default:0"`
	Status        string    `gorm:"column:status;size:16;not null;default:published"`
	RequestID     string    `gorm:"column:request_id;size:64"`
	CreateTime    time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)" json:"created_at"`
	UpdateTime    time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP(3)" json:"updated_at"`
}

// InfoCacheEntry 视频详情缓存里的一条记录
type InfoCacheEntry struct {
	ID            uint64    `json:"id"`
	AuthorID      uint64    `json:"author_id"`
	AuthorName    string    `json:"author_name"`
	AuthorAvatar  string    `json:"author_avatar"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	CoverURL      string    `json:"cover_url"`
	PlayURL       string    `json:"play_url"`
	LikeCount     int64     `json:"like_count"`
	CommentCount  int64     `json:"comment_count"`
	FavoriteCount int64     `json:"favorite_count"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// InfoCache 带逻辑过期时间的详情缓存记录
type InfoCache struct {
	Entry    *InfoCacheEntry `json:"data,omitempty"`
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

// GetByRequestID 按请求 ID 查作者的发布记录，用于重复创建判重
func (r *Repo) GetByRequestID(ctx context.Context, authorID uint64, requestID string) (*Video, error) {
	var item Video
	if err := r.db.WithContext(ctx).Where("author_id = ? AND request_id = ?", authorID, requestID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// FilterByIDs 按 ID 批量取已发布的视频，供热榜按有序集合给出的顺序回表
func (r *Repo) FilterByIDs(ctx context.Context, videoIDs []uint64) ([]Video, error) {
	if len(videoIDs) == 0 {
		return []Video{}, nil
	}
	items := make([]Video, 0, len(videoIDs))
	if err := r.db.WithContext(ctx).
		Where("id in ? and status = ?", videoIDs, StatusPublished).
		Find(&items).Error; err != nil {
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

// ListIDsByAuthor 按作者列出最近发布的视频 ID，供资料变更时批量清理缓存
func (r *Repo) ListIDsByAuthor(ctx context.Context, authorID uint64, limit int) ([]uint64, error) {
	ids := make([]uint64, 0, limit)
	err := r.db.WithContext(ctx).Model(&Video{}).
		Where("author_id = ?", authorID).
		Order("id desc").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

func (r *Repo) CountByAuthor(ctx context.Context, authorID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Video{}).
		Where("author_id = ? AND status = ?", authorID, StatusPublished).Count(&count).Error
	return count, err
}

// ListByAuthorsBefore 关注流分页：作者集合内按发布时间倒序，双字段游标定位，第一页 lastID 传 0
func (r *Repo) ListByAuthorsBefore(ctx context.Context, authorIDs []uint64, limit uint64, lastCreatedAt time.Time, lastID uint64) ([]Video, error) {
	if len(authorIDs) == 0 {
		return []Video{}, nil
	}
	items := make([]Video, 0, limit)
	query := r.db.WithContext(ctx).Model(&Video{}).Where("author_id in ? AND status = ?", authorIDs, StatusPublished)
	if lastID > 0 {
		query = query.Where("created_at <= ? AND id < ?", lastCreatedAt, lastID)
	}
	err := query.Order("created_at desc, id desc").Limit(int(limit)).Find(&items).Error
	return items, err
}

// ListFeedPage 推荐流分页：全表按发布时间倒序，双字段游标定位，第一页 lastID 传 0
func (r *Repo) ListFeedPage(ctx context.Context, limit uint64, lastCreatedAt time.Time, lastID uint64) ([]Video, error) {
	items := make([]Video, 0, limit)
	query := r.db.WithContext(ctx).Model(&Video{}).Where("status = ?", StatusPublished)
	if lastID > 0 {
		query = query.Where("created_at <= ? AND id < ?", lastCreatedAt, lastID)
	}
	err := query.Order("created_at desc, id desc").Limit(int(limit)).Find(&items).Error
	return items, err
}

func (r *Repo) Delete(ctx context.Context, videoID uint64) error {
	return r.db.WithContext(ctx).Delete(&Video{}, videoID).Error
}

// MarkStatus 条件更新发布状态，只允许从 from 迁移到 to，返回是否发生了迁移
func (r *Repo) MarkStatus(ctx context.Context, videoID uint64, authorID uint64, from string, to string) (bool, error) {
	result := r.db.WithContext(ctx).Model(&Video{}).
		Where("id = ? AND author_id = ? AND status = ?", videoID, authorID, from).
		Update("status", to)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// SyncLikeCount 按实际点赞数对账视频点赞数
func (r *Repo) SyncLikeCount(ctx context.Context, videoID uint64, count int64) error {
	return r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", videoID).
		Update("like_count", count).Error
}

// SyncFavoriteCount 按实际收藏数对账视频收藏数
func (r *Repo) SyncFavoriteCount(ctx context.Context, videoID uint64, count int64) error {
	return r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", videoID).
		Update("favorite_count", count).Error
}

// UpdateAuthorInfo 刷新某个作者全部视频上冗余的展示字段
func (r *Repo) UpdateAuthorInfo(ctx context.Context, authorID uint64, authorName string, authorAvatar string) error {
	updates := map[string]any{"author_avatar": authorAvatar}
	if authorName != "" {
		updates["author_name"] = authorName
	}
	return r.db.WithContext(ctx).Model(&Video{}).Where("author_id = ?", authorID).Updates(updates).Error
}

// GetInfoCache 读详情缓存，返回三种状态：
// found 为 false 表示键不存在，需要回源；
// found 为 true 且 record 为 nil 表示空值标记，MySQL 里没有这一行；
// found 为 true 且 record 非 nil 表示缓存命中。
func (r *Repo) GetInfoCache(ctx context.Context, videoID uint64) (*InfoCache, bool, error) {
	raw, err := r.redisClient.Get(ctx, infoCacheKey(videoID)).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if raw == "" {
		return nil, true, nil
	}
	var record InfoCache
	if unmarshalErr := json.Unmarshal([]byte(raw), &record); unmarshalErr != nil {
		_ = r.redisClient.Del(ctx, infoCacheKey(videoID)).Err()
		return nil, false, nil
	}
	if record.Entry == nil {
		// Entry 为 nil 的历史空值记录按空值标记处理
		return nil, true, nil
	}
	return &record, true, nil
}

// SetInfoCache 写入视频信息缓存，实际有效期远长于记录里的逻辑过期时间
func (r *Repo) SetInfoCache(ctx context.Context, videoID uint64, cache InfoCache) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return r.redisClient.Set(ctx, infoCacheKey(videoID), data, infoPhysicalTTL).Err()
}

// SetInfoMiss 键不存在时写空值标记，纯物理过期，不需要逻辑过期
func (r *Repo) SetInfoMiss(ctx context.Context, videoID uint64) error {
	return r.redisClient.Set(ctx, infoCacheKey(videoID), "", infoMissTTL).Err()
}

// DeleteInfoCache 让详情缓存失效
func (r *Repo) DeleteInfoCache(ctx context.Context, videoID uint64) error {
	return r.redisClient.Del(ctx, infoCacheKey(videoID)).Err()
}

// DeleteInfoCacheBatch 批量删除详情缓存，用于资料变更时清理该作者全部视频
func (r *Repo) DeleteInfoCacheBatch(ctx context.Context, videoIDs []uint64) error {
	if len(videoIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		keys = append(keys, infoCacheKey(videoID))
	}
	return r.redisClient.Del(ctx, keys...).Err()
}

func infoCacheKey(videoID uint64) string {
	return fmt.Sprintf(infoCacheKeyFormat, videoID)
}
