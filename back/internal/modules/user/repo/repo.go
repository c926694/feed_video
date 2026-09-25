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
	CreatedAt     time.Time `gorm:"autoCreateTime;not null"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime;not null"`
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

func (r *Repo) IncreaseFollowCount(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Update("follow_count", gorm.Expr("follow_count + 1")).Error
}

func (r *Repo) DecreaseFollowCount(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Update("follow_count", gorm.Expr("CASE WHEN follow_count > 0 THEN follow_count - 1 ELSE 0 END")).Error
}

func (r *Repo) IncreaseFollowerCount(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Update("follower_count", gorm.Expr("follower_count + 1")).Error
}

func (r *Repo) DecreaseFollowerCount(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Update("follower_count", gorm.Expr("CASE WHEN follower_count > 0 THEN follower_count - 1 ELSE 0 END")).Error
}

func (r *Repo) IncreaseVideoCount(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Update("video_count", gorm.Expr("video_count + 1")).Error
}

func (r *Repo) DecreaseVideoCount(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).
		Update("video_count", gorm.Expr("CASE WHEN video_count > 0 THEN video_count - 1 ELSE 0 END")).Error
}
