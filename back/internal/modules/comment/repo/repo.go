package repo

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Comment comment 表
type Comment struct {
	ID              uint64    `gorm:"primaryKey"`
	Commenter       uint64    `gorm:"commenter" `
	CommenterName   string    `gorm:"size:64"`
	CommenterAvatar string    `gorm:"size:255"`
	VideoID         uint64    `gorm:"index;not null"`
	Content         string    `gorm:"size:500;not null"`
	LikeCount       int64     `gorm:"default:0"`
	ParentID        uint64    `gorm:"parent_id;default:0"`
	ReplyToID       uint64    `gorm:"reply_to_id;default:0"`
	ReplyToUserID   uint64    `gorm:"reply_to_user_id;default:0"`
	ReplyToUserName string    `gorm:"reply_to_user_name;size:64"`
	CreateTime      time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)" json:"created_at"`
	UpdateTime      time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP(3)" json:"updated_at"`
}

// Repo comment 表的读写
type Repo struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(ctx context.Context, item *Comment) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repo) GetByID(ctx context.Context, commentID uint64) (*Comment, error) {
	var item Comment
	if err := r.db.WithContext(ctx).Where("id = ?", commentID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// ListTopPage 顶级评论分页：双字段游标，倒序
func (r *Repo) ListTopPage(ctx context.Context, videoID uint64, lastCreatedAt time.Time, lastID uint64, limit int) ([]Comment, error) {
	items := make([]Comment, 0, limit)
	query := r.db.WithContext(ctx).Where("video_id = ? AND parent_id = 0", videoID)
	if lastID > 0 {
		query = query.Where("created_at <= ? AND id < ?", lastCreatedAt, lastID)
	}
	if err := query.Order("created_at desc, id desc").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListRepliesPage 某个顶级评论下的子评论分页：双字段游标，倒序
func (r *Repo) ListRepliesPage(ctx context.Context, videoID uint64, parentID uint64, lastCreatedAt time.Time, lastID uint64, limit int) ([]Comment, error) {
	items := make([]Comment, 0, limit)
	query := r.db.WithContext(ctx).Where("video_id = ? AND parent_id = ?", videoID, parentID)
	if lastID > 0 {
		query = query.Where("created_at <= ? AND id < ?", lastCreatedAt, lastID)
	}
	if err := query.Order("created_at desc, id desc").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListRecentReplies 某个顶级评论下最新的若干条子评论，返回正序供展示
func (r *Repo) ListRecentReplies(ctx context.Context, videoID uint64, parentID uint64, limit int) ([]Comment, error) {
	items := make([]Comment, 0, limit)
	if err := r.db.WithContext(ctx).
		Where("video_id = ? AND parent_id = ?", videoID, parentID).
		Order("created_at desc, id desc").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items, nil
}

// ListRepliesAll 某个顶级评论下的全部子评论，删除整楼时使用
func (r *Repo) ListRepliesAll(ctx context.Context, parentID uint64) ([]Comment, error) {
	items := make([]Comment, 0)
	if err := r.db.WithContext(ctx).Where("parent_id = ?", parentID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repo) Delete(ctx context.Context, commentID uint64) error {
	return r.db.WithContext(ctx).Where("id = ?", commentID).Delete(&Comment{}).Error
}

func (r *Repo) DeleteByVideo(ctx context.Context, videoID uint64) error {
	return r.db.WithContext(ctx).Where("video_id = ?", videoID).Delete(&Comment{}).Error
}

// CountByVideo 统计某个视频下的全部评论数，子评论也算
func (r *Repo) CountByVideo(ctx context.Context, videoID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Comment{}).Where("video_id = ?", videoID).Count(&count).Error
	return count, err
}

// SyncLikeCount 按实际点赞数对账评论点赞数
func (r *Repo) SyncLikeCount(ctx context.Context, commentID uint64, count int64) error {
	return r.db.WithContext(ctx).Model(&Comment{}).Where("id = ?", commentID).
		Update("like_count", count).Error
}

// UpdateCommenterInfo 刷新某个评论者全部评论上冗余的展示字段
func (r *Repo) UpdateCommenterInfo(ctx context.Context, commenterID uint64, commenterName string, commenterAvatar string) error {
	updates := map[string]any{"commenter_avatar": commenterAvatar}
	if commenterName != "" {
		updates["commenter_name"] = commenterName
	}
	return r.db.WithContext(ctx).Model(&Comment{}).Where("commenter = ?", commenterID).Updates(updates).Error
}
