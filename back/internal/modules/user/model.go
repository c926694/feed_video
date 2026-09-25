package user

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime/multipart"
	"strconv"
	"strings"

	"gorm.io/gorm"

	followevent "simple_tiktok/internal/modules/follow/event"
	userevent "simple_tiktok/internal/modules/user/event"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videoevent "simple_tiktok/internal/modules/video/event"
	"simple_tiktok/internal/pkg/hash_password"
	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/platform/upload"
)

// Model 用户模块的业务逻辑。视频数量、关注计数都保存在自己的表里，由事件维护。
type Model struct {
	users    *userrepo.Repo
	auth     *auth.Service
	uploader *upload.Uploader
	producer *producer.Producer
}

func NewModel(users *userrepo.Repo, authService *auth.Service, uploader *upload.Uploader, eventProducer *producer.Producer) *Model {
	return &Model{
		users:    users,
		auth:     authService,
		uploader: uploader,
		producer: eventProducer,
	}
}

func (m *Model) Register(ctx context.Context, registerReq RegisterReq) (uint64, error) {
	if registerReq.Password != registerReq.RePassword {
		return 0, httpx.New(httpx.CodeBadRequest, "两次输入的密码不一致")
	}
	if err := checkValidUsernameAndPassword(registerReq.Username, registerReq.Password); err != nil {
		return 0, err
	}
	hashed, err := hash_password.HashPassword(registerReq.Password)
	if err != nil {
		return 0, err
	}
	item, err := m.users.Create(ctx, registerReq.Username, hashed, userrepo.DefaultAvatar)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return 0, httpx.New(httpx.CodeConflict, "用户名已存在")
		}
		return 0, err
	}
	return item.ID, nil
}

func (m *Model) Login(ctx context.Context, username string, password string) (string, error) {
	if err := checkValidUsernameAndPassword(username, password); err != nil {
		return "", err
	}
	item, err := m.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", httpx.New(httpx.CodeCredential, "用户名或密码错误")
		}
		return "", err
	}
	if !hash_password.CheckPassword(password, item.Password) {
		return "", httpx.New(httpx.CodeCredential, "用户名或密码错误")
	}
	return m.auth.GenerateToken(ctx, item.ID, item.NickName)
}

func (m *Model) GetInfo(ctx context.Context, userID uint64) (*InfoRes, error) {
	item, err := m.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.New(httpx.CodeNotFound, "用户不存在")
		}
		return nil, err
	}
	return m.toInfoRes(item), nil
}

func (m *Model) UpdateProfile(ctx context.Context, userID uint64, nickname string, avatar *multipart.FileHeader) (*InfoRes, error) {
	item, err := m.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.New(httpx.CodeNotFound, "用户不存在")
		}
		return nil, err
	}

	updates := make(map[string]any)
	nickname = strings.TrimSpace(nickname)
	if nickname != "" {
		updates["nick_name"] = nickname
	}

	newAvatarPath := ""
	if avatar != nil {
		newAvatarPath, err = m.uploader.Save(avatar, upload.Avatar)
		if err != nil {
			return nil, err
		}
		updates["avatar_url"] = newAvatarPath
	}

	if err = m.users.UpdateProfile(ctx, userID, updates); err != nil {
		if newAvatarPath != "" {
			_ = m.uploader.Delete(upload.Avatar, newAvatarPath)
		}
		return nil, err
	}

	if newAvatarPath != "" && item.AvatarURL != "" && item.AvatarURL != userrepo.DefaultAvatar {
		_ = m.uploader.Delete(upload.Avatar, item.AvatarURL)
	}

	updated, err := m.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 通知 video 与 comment 刷新自己表里冗余的作者展示字段
	if err = m.producer.Publish(ctx, topic.UserUpdated, strconv.FormatUint(userID, 10), userevent.UpdatedEvent{
		UserID:    userID,
		Nickname:  updated.NickName,
		AvatarURL: updated.AvatarURL,
	}); err != nil {
		return nil, err
	}

	return m.toInfoRes(updated), nil
}

func (m *Model) Logout(ctx context.Context, userID uint64) error {
	return m.auth.Revoke(ctx, userID)
}

// HandleFollowSwitched 订阅关注事件，维护 user 表上的两个关注计数
func (m *Model) HandleFollowSwitched(ctx context.Context, payload []byte) error {
	var switched followevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Follower == 0 || switched.Following == 0 {
		return consumer.Permanent(errors.New("关注事件里缺少用户 ID"))
	}

	var err error
	if switched.Followed {
		if err = m.users.IncreaseFollowCount(ctx, switched.Follower); err == nil {
			err = m.users.IncreaseFollowerCount(ctx, switched.Following)
		}
	} else {
		if err = m.users.DecreaseFollowCount(ctx, switched.Follower); err == nil {
			err = m.users.DecreaseFollowerCount(ctx, switched.Following)
		}
	}
	if err != nil {
		slog.Error("更新关注计数失败", "follower", switched.Follower, "following", switched.Following, "error", err)
		return err
	}
	return nil
}

// HandleVideoCreated 订阅视频创建事件，视频数量加一
func (m *Model) HandleVideoCreated(ctx context.Context, payload []byte) error {
	var created videoevent.CreatedEvent
	if err := json.Unmarshal(payload, &created); err != nil {
		return consumer.Permanent(err)
	}
	if created.AuthorID == 0 {
		return consumer.Permanent(errors.New("视频创建事件里没有 authorId"))
	}
	if err := m.users.IncreaseVideoCount(ctx, created.AuthorID); err != nil {
		slog.Error("更新视频数量失败", "author_id", created.AuthorID, "error", err)
		return err
	}
	return nil
}

// HandleVideoDeleted 订阅视频删除事件，视频数量减一
func (m *Model) HandleVideoDeleted(ctx context.Context, payload []byte) error {
	var deleted videoevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.AuthorID == 0 {
		return consumer.Permanent(errors.New("删除视频事件里没有 authorId"))
	}
	if err := m.users.DecreaseVideoCount(ctx, deleted.AuthorID); err != nil {
		slog.Error("更新视频数量失败", "author_id", deleted.AuthorID, "error", err)
		return err
	}
	return nil
}

func (m *Model) toInfoRes(item *userrepo.User) *InfoRes {
	return &InfoRes{
		UserID:        item.ID,
		Username:      item.Username,
		Nickname:      item.NickName,
		AvatarURL:     m.uploader.URL(item.AvatarURL),
		FollowCount:   item.FollowCount,
		FollowerCount: item.FollowerCount,
		VideoCount:    item.VideoCount,
	}
}

func checkValidUsernameAndPassword(username string, password string) error {
	if username == "" {
		return httpx.New(httpx.CodeBadRequest, "用户名不能为空")
	}
	if password == "" {
		return httpx.New(httpx.CodeBadRequest, "密码不能为空")
	}
	return nil
}
