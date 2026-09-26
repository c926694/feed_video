package repo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"simple_tiktok/internal/pkg/random_nickname"
)

// DefaultAvatar 新用户的默认头像路径
const DefaultAvatar = "avatar/default.svg"

// User user 表
type User struct {
	ID            uint64    `gorm:"primaryKey"`
	Username      string    `gorm:"size:64;uniqueIndex;not null"`
	Password      string    `gorm:"size:128;not null"`
	NickName      string    `gorm:"size:64;not null"`
	AvatarURL     string    `gorm:"size:255"`
	FollowCount   int64     `gorm:"default:0"`
	FollowerCount int64     `gorm:"default:0"`
	VideoCount    int64     `gorm:"default:0"`
	CreateTime    time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)" json:"created_at"`
	UpdateTime    time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP(3)" json:"updated_at"`
}

// Repo user 表的读写
type Repo struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(ctx context.Context, username string, hashedPassword string, avatarURL string) (*User, error) {
	item := User{
		Username:  username,
		Password:  hashedPassword,
		NickName:  random_nickname.GenerateNickname(),
		AvatarURL: avatarURL,
	}
	if err := r.db.WithContext(ctx).Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repo) GetByID(ctx context.Context, userID uint64) (*User, error) {
	var item User
	if err := r.db.WithContext(ctx).Where("id = ?", userID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repo) GetByUsername(ctx context.Context, username string) (*User, error) {
	var item User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repo) FilterByIDs(ctx context.Context, userIDs []uint64) ([]User, error) {
	if len(userIDs) == 0 {
		return []User{}, nil
	}
	items := make([]User, 0, len(userIDs))
	if err := r.db.WithContext(ctx).Where("id in ?", userIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repo) UpdateProfile(ctx context.Context, userID uint64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Updates(updates).Error
}

// SyncFollowCount 按实际关注数对账
func (r *Repo) SyncFollowCount(ctx context.Context, userID uint64, count int64) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Update("follow_count", count).Error
}

// SyncFollowerCount 按实际粉丝数对账
func (r *Repo) SyncFollowerCount(ctx context.Context, userID uint64, count int64) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Update("follower_count", count).Error
}

// SyncVideoCount 按实际视频数对账
func (r *Repo) SyncVideoCount(ctx context.Context, userID uint64, count int64) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Update("video_count", count).Error
}
