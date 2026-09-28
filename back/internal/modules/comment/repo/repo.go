package repo

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	ReplyCount      int64     `gorm:"reply_count;default:0"`
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

// CreateTop 创建顶级评论，并在同一事务里累加视频评论数。
// 评论相关的计数全部由写路径事务同步维护，不依赖事件
func (r *Repo) CreateTop(ctx context.Context, item *Comment, videoID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		return incrVideoCommentCount(tx, videoID, 1)
	})
}

// CreateReply 创建子评论：锁定父评论（顺带完成存在性校验），
// 同一事务里插入回复、累加父评论回复数与视频评论数。
// 锁父行让「回复创建」与「整楼删除」串行，不会产生孤儿回复
func (r *Repo) CreateReply(ctx context.Context, item *Comment, parentID uint64, videoID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var parent Comment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", parentID).First(&parent).Error; err != nil {
			return err
		}
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		if err := tx.Model(&Comment{}).Where("id = ?", parentID).
			UpdateColumn("reply_count", gorm.Expr("reply_count + 1")).Error; err != nil {
			return err
		}
		return incrVideoCommentCount(tx, videoID, 1)
	})
}

// DeleteReply 删除子评论，并在同一事务里扣减父评论回复数与视频评论数
func (r *Repo) DeleteReply(ctx context.Context, commentID uint64, parentID uint64, videoID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", commentID).Delete(&Comment{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Comment{}).Where("id = ? AND reply_count > 0", parentID).
			UpdateColumn("reply_count", gorm.Expr("reply_count - 1")).Error; err != nil {
			return err
		}
		return incrVideoCommentCount(tx, videoID, -1)
	})
}

// DeleteTopWithReplies 删除整楼：锁定楼主行，删掉楼内全部评论，
// 按实际删除的行数扣减视频评论数，返回被删的行供外层清理点赞关系
func (r *Repo) DeleteTopWithReplies(ctx context.Context, commentID uint64, videoID uint64) ([]Comment, error) {
	var removed []Comment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var top Comment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", commentID).First(&top).Error; err != nil {
			return err
		}
		removed = []Comment{top}

		children := make([]Comment, 0)
		if err := tx.Where("parent_id = ?", commentID).Find(&children).Error; err != nil {
			return err
		}
		removed = append(removed, children...)

		if err := tx.Where("id = ? OR parent_id = ?", commentID, commentID).
			Delete(&Comment{}).Error; err != nil {
			return err
		}
		return incrVideoCommentCount(tx, videoID, -int64(len(removed)))
	})
	if err != nil {
		return nil, err
	}
	return removed, nil
}

// incrVideoCommentCount 在事务里原子增减视频评论数，减时不为负
func incrVideoCommentCount(tx *gorm.DB, videoID uint64, delta int64) error {
	expr := "comment_count + ?"
	if delta < 0 {
		expr = "GREATEST(comment_count + ?, 0)"
	}
	return tx.Table("video").Where("id = ?", videoID).
		UpdateColumn("comment_count", gorm.Expr(expr, delta)).Error
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

// DeleteByVideo 视频删除时清理它下面的全部评论，不需要维护任何计数
func (r *Repo) DeleteByVideo(ctx context.Context, videoID uint64) error {
	return r.db.WithContext(ctx).Where("video_id = ?", videoID).Delete(&Comment{}).Error
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
