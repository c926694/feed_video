package repo

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 通知类型
const (
	TypeLikeVideo   = "like_video"
	TypeLikeComment = "like_comment"
	TypeComment     = "comment"
	TypeReply       = "reply"
	TypeFollow      = "follow"
)

// ActorIDsLimit 会话行上保留的参与者个数
const ActorIDsLimit = 3

// Message 通知明细，一笔互动一行
type Message struct {
	ID        uint64    `gorm:"primaryKey"`
	UserID    uint64    `gorm:"not null"`
	ThreadID  uint64    `gorm:"not null;default:0"`
	Type      string    `gorm:"size:16;not null"`
	TargetID  uint64    `gorm:"not null"`
	ActorID   uint64    `gorm:"not null"`
	VideoID   uint64    `gorm:"not null;default:0"`
	Content   string    `gorm:"size:255;not null;default:''"`
	CreatedAt time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)"`
}

// MessageThread 通知会话，列表与已读都以它为准
type MessageThread struct {
	ID           uint64     `gorm:"primaryKey"`
	UserID       uint64     `gorm:"not null"`
	Type         string     `gorm:"size:16;not null"`
	TargetID     uint64     `gorm:"not null"`
	WindowBucket uint32     `gorm:"not null"`
	VideoID      uint64     `gorm:"not null;default:0"`
	ActorCount   uint32     `gorm:"not null;default:1"`
	ActorIDs     string     `gorm:"size:255;not null;default:''"`
	Content      string     `gorm:"size:255;not null;default:''"`
	IsRead       bool       `gorm:"not null;default:false"`
	ReadAt       *time.Time `gorm:"column:read_at"`
	LastAt       time.Time  `gorm:"column:last_at;not null"`
	CreatedAt    time.Time  `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)"`
}

// CommentIDOf 评论类通知的评论 ID 就是 target_id，其余类型没有评论目标
func CommentIDOf(msgType string, targetID uint64) uint64 {
	switch msgType {
	case TypeLikeComment, TypeComment, TypeReply:
		return targetID
	default:
		return 0
	}
}

// Repo message 与 message_thread 两张表的读写
type Repo struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// InsertDetail 写入一条通知明细，返回明细 ID 与是否是新插入的。
// 唯一键冲突说明这笔互动已经通知过，返回 false，调用方不再动会话行
func (r *Repo) InsertDetail(ctx context.Context, item *Message) (uint64, bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(item)
	if result.Error != nil {
		return 0, false, result.Error
	}
	return item.ID, result.RowsAffected == 1, nil
}

// UpsertThread 累加会话：人数加一、参与者列表把新的人插到最前并截断、
// 刷新最后互动时间，并把会话重新置为未读。
// 用 MySQL 的 values() 引用本次要插入的值，所以不需要先读一次旧行
func (r *Repo) UpsertThread(ctx context.Context, item *MessageThread) error {
	const query = `
INSERT INTO message_thread
  (user_id, type, target_id, window_bucket, video_id, actor_count, actor_ids, content, is_read, last_at)
VALUES (?, ?, ?, ?, ?, 1, ?, ?, 0, ?)
ON DUPLICATE KEY UPDATE
  actor_count = actor_count + 1,
  actor_ids   = substring_index(concat(values(actor_ids), if(actor_ids = '', '', concat(',', actor_ids))), ',', 3),
  content     = values(content),
  last_at     = values(last_at),
  is_read     = 0,
  read_at     = NULL`
	return r.db.WithContext(ctx).Exec(query,
		item.UserID, item.Type, item.TargetID, item.WindowBucket, item.VideoID,
		item.ActorIDs, item.Content, item.LastAt).Error
}

// BindThread 把明细行挂到它所属的会话上，供将来展开参与者使用
func (r *Repo) BindThread(ctx context.Context, messageID uint64, item *MessageThread) error {
	const query = `
UPDATE message SET thread_id = (
  SELECT id FROM message_thread WHERE user_id = ? AND type = ? AND target_id = ? AND window_bucket = ?
) WHERE id = ?`
	return r.db.WithContext(ctx).Exec(query,
		item.UserID, item.Type, item.TargetID, item.WindowBucket, messageID).Error
}

// ListThreads 按最后一次互动时间倒序取一页会话，双字段游标定位
func (r *Repo) ListThreads(ctx context.Context, userID uint64, lastAt time.Time, lastID uint64, limit int) ([]MessageThread, error) {
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if lastID > 0 {
		query = query.Where("last_at < ? OR (last_at = ? AND id < ?)", lastAt, lastAt, lastID)
	}
	items := make([]MessageThread, 0, limit)
	err := query.Order("last_at desc").Order("id desc").Limit(limit).Find(&items).Error
	return items, err
}

// CountUnread 统计未读会话数
func (r *Repo) CountUnread(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&MessageThread{}).
		Where("user_id = ? AND is_read = ?", userID, false).Count(&count).Error
	return count, err
}

// MarkRead 把指定会话标为已读，messageIDs 为空表示全部。
// 条件里带 is_read = false，重复调用不会覆盖第一次的 read_at
func (r *Repo) MarkRead(ctx context.Context, userID uint64, messageIDs []uint64) (int64, error) {
	now := time.Now()
	query := r.db.WithContext(ctx).Model(&MessageThread{}).
		Where("user_id = ? AND is_read = ?", userID, false)
	if len(messageIDs) > 0 {
		query = query.Where("id IN ?", messageIDs)
	}
	result := query.Updates(map[string]any{"is_read": true, "read_at": now})
	return result.RowsAffected, result.Error
}

// Cleanup 删除已读且最后互动时间早于 before 的会话，再删它们的明细。
// 先删会话再删明细，顺序不能反，否则明细会成为孤儿
func (r *Repo) Cleanup(ctx context.Context, before time.Time, batch int) (int, error) {
	if batch <= 0 {
		batch = 500
	}
	deleted := 0
	for {
		ids := make([]uint64, 0, batch)
		if err := r.db.WithContext(ctx).Model(&MessageThread{}).
			Where("is_read = ? AND last_at < ?", true, before).
			Order("id asc").Limit(batch).Pluck("id", &ids).Error; err != nil {
			return deleted, err
		}
		if len(ids) == 0 {
			return deleted, nil
		}
		if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("thread_id IN ?", ids).Delete(&Message{}).Error; err != nil {
				return err
			}
			return tx.Where("id IN ?", ids).Delete(&MessageThread{}).Error
		}); err != nil {
			return deleted, err
		}
		deleted += len(ids)
		if len(ids) < batch {
			return deleted, nil
		}
	}
}

// SplitActorIDs 解析会话行上的参与者列表
func SplitActorIDs(value string) []uint64 {
	if value == "" {
		return nil
	}
	ids := make([]uint64, 0, ActorIDsLimit)
	for _, part := range strings.Split(value, ",") {
		id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// LikeRow 点赞关系行，补齐通知时作为来源数据
type LikeRow struct {
	ID         uint64
	UserID     uint64
	TargetType string
	TargetID   uint64
	CreatedAt  time.Time `gorm:"column:created_at"`
}

// CommentRow 评论行，补齐通知时作为来源数据
type CommentRow struct {
	ID        uint64
	CreatedAt time.Time `gorm:"column:created_at"`
}

// FollowRow 关注关系行，补齐通知时作为来源数据
type FollowRow struct {
	ID        uint64
	Follower  uint64
	Following uint64
	CreatedAt time.Time `gorm:"column:created_at"`
}

// PageLikes 按自增 ID 取一页点赞关系
func (r *Repo) PageLikes(ctx context.Context, afterID uint64, limit int) ([]LikeRow, error) {
	rows := make([]LikeRow, 0, limit)
	err := r.db.WithContext(ctx).Table("user_like").
		Select("id, user_id, target_type, target_id, created_at").
		Where("id > ?", afterID).Order("id asc").Limit(limit).Scan(&rows).Error
	return rows, err
}

// PageComments 按自增 ID 取一页评论
func (r *Repo) PageComments(ctx context.Context, afterID uint64, limit int) ([]CommentRow, error) {
	rows := make([]CommentRow, 0, limit)
	err := r.db.WithContext(ctx).Table("comment").
		Select("id, created_at").
		Where("id > ?", afterID).Order("id asc").Limit(limit).Scan(&rows).Error
	return rows, err
}

// PageFollows 按自增 ID 取一页关注关系
func (r *Repo) PageFollows(ctx context.Context, afterID uint64, limit int) ([]FollowRow, error) {
	rows := make([]FollowRow, 0, limit)
	err := r.db.WithContext(ctx).Table("follow").
		Select("id, follower, following, created_at").
		Where("id > ?", afterID).Order("id asc").Limit(limit).Scan(&rows).Error
	return rows, err
}

// ExistingUserIDs 取目前存在的用户 ID，补齐通知时跳过已经没有账号的接收者
func (r *Repo) ExistingUserIDs(ctx context.Context) (map[uint64]bool, error) {
	ids := make([]uint64, 0, 256)
	if err := r.db.WithContext(ctx).Table("user").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	existing := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		existing[id] = true
	}
	return existing, nil
}
