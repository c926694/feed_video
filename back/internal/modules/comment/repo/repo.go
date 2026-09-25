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
	CreatedAt       time.Time `gorm:"autoCreateTime;not null"`
	UpdatedAt       time.Time `gorm:" autoUpdateTime;not null"`
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

func (r *Repo) ListByVideo(ctx context.Context, videoID uint64) ([]Comment, error) {
	items := make([]Comment, 0)
	if err := r.db.WithContext(ctx).Where("video_id = ?", videoID).Find(&items).Error; err != nil {
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

func (r *Repo) IncreaseLikeCount(ctx context.Context, commentID uint64) error {
	return r.db.WithContext(ctx).Model(&Comment{}).Where("id = ?", commentID).
		Update("like_count", gorm.Expr("like_count + 1")).Error
}

func (r *Repo) DecreaseLikeCount(ctx context.Context, commentID uint64) error {
	return r.db.WithContext(ctx).Model(&Comment{}).Where("id = ?", commentID).
		Update("like_count", gorm.Expr("CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END")).Error
}

// UpdateCommenterInfo 刷新某个评论者全部评论上冗余的展示字段
func (r *Repo) UpdateCommenterInfo(ctx context.Context, commenterID uint64, commenterName string, commenterAvatar string) error {
	updates := map[string]any{"commenter_avatar": commenterAvatar}
	if commenterName != "" {
		updates["commenter_name"] = commenterName
	}
	return r.db.WithContext(ctx).Model(&Comment{}).Where("commenter = ?", commenterID).Updates(updates).Error
}
