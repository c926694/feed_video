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

// Logic 用户模块的业务逻辑。视频数量、关注计数都保存在自己的表里，由事件维护。
type Logic struct {
	users    *userrepo.Repo
	auth     *auth.Service
	uploader *upload.Uploader
	producer *producer.Producer
}

func (l *Logic) Register(ctx context.Context, registerReq RegisterReq) (uint64, error) {
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
	item, err := l.users.Create(ctx, registerReq.Username, hashed, userrepo.DefaultAvatar)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return 0, httpx.New(httpx.CodeConflict, "用户名已存在")
		}
		return 0, err
	}
	return item.ID, nil
}

func (l *Logic) Login(ctx context.Context, username string, password string) (auth.TokenPair, error) {
	if err := checkValidUsernameAndPassword(username, password); err != nil {
		return auth.TokenPair{}, err
	}
	item, err := l.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return auth.TokenPair{}, httpx.New(httpx.CodeCredential, "用户名或密码错误")
		}
		return auth.TokenPair{}, err
	}
	if !hash_password.CheckPassword(password, item.Password) {
		return auth.TokenPair{}, httpx.New(httpx.CodeCredential, "用户名或密码错误")
	}
	return l.auth.Issue(ctx, item.ID)
}

func (l *Logic) GetInfo(ctx context.Context, userID uint64) (*InfoRes, error) {
	item, err := l.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.New(httpx.CodeNotFound, "用户不存在")
		}
		return nil, err
	}
	return l.toInfoRes(item), nil
}

func (l *Logic) UpdateProfile(ctx context.Context, userID uint64, nickname string, avatar *multipart.FileHeader) (*InfoRes, error) {
	item, err := l.users.GetByID(ctx, userID)
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
		newAvatarPath, err = l.uploader.Save(avatar, upload.Avatar)
		if err != nil {
			return nil, err
		}
		updates["avatar_url"] = newAvatarPath
	}

	if err = l.users.UpdateProfile(ctx, userID, updates); err != nil {
		if newAvatarPath != "" {
			_ = l.uploader.Delete(upload.Avatar, newAvatarPath)
		}
		return nil, err
	}

	if newAvatarPath != "" && item.AvatarURL != "" && item.AvatarURL != userrepo.DefaultAvatar {
		_ = l.uploader.Delete(upload.Avatar, item.AvatarURL)
	}

	updated, err := l.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 通知 video 与 comment 刷新自己表里冗余的作者展示字段
	if err = l.producer.Publish(ctx, topic.UserUpdated, strconv.FormatUint(userID, 10), userevent.UpdatedEvent{
		UserID:    userID,
		Nickname:  updated.NickName,
		AvatarURL: updated.AvatarURL,
	}); err != nil {
		return nil, err
	}

	return l.toInfoRes(updated), nil
}

// Refresh 用 refresh token 换一对新令牌
func (l *Logic) Refresh(ctx context.Context, refreshToken string) (auth.TokenPair, error) {
	return l.auth.Refresh(ctx, refreshToken)
}

// Logout 结束当前这条会话，其他设备的登录状态不受影响
func (l *Logic) Logout(ctx context.Context, refreshToken string) error {
	return l.auth.Revoke(ctx, refreshToken)
}

// HandleFollowSwitched 订阅关注事件，维护 user 表上的两个关注计数
func (l *Logic) HandleFollowSwitched(ctx context.Context, payload []byte) error {
	var switched followevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Follower == 0 || switched.Following == 0 {
		return consumer.Permanent(errors.New("关注事件里缺少用户 ID"))
	}

	var err error
	if switched.Followed {
		if err = l.users.IncreaseFollowCount(ctx, switched.Follower); err == nil {
			err = l.users.IncreaseFollowerCount(ctx, switched.Following)
		}
	} else {
		if err = l.users.DecreaseFollowCount(ctx, switched.Follower); err == nil {
			err = l.users.DecreaseFollowerCount(ctx, switched.Following)
		}
	}
	if err != nil {
		slog.Error("更新关注计数失败", "follower", switched.Follower, "following", switched.Following, "error", err)
		return err
	}
	return nil
}

// HandleVideoCreated 订阅视频创建事件，视频数量加一
func (l *Logic) HandleVideoCreated(ctx context.Context, payload []byte) error {
	var created videoevent.CreatedEvent
	if err := json.Unmarshal(payload, &created); err != nil {
		return consumer.Permanent(err)
	}
	if created.AuthorID == 0 {
		return consumer.Permanent(errors.New("视频创建事件里没有 authorId"))
	}
	if err := l.users.IncreaseVideoCount(ctx, created.AuthorID); err != nil {
		slog.Error("更新视频数量失败", "author_id", created.AuthorID, "error", err)
		return err
	}
	return nil
}

// HandleVideoDeleted 订阅视频删除事件，视频数量减一
func (l *Logic) HandleVideoDeleted(ctx context.Context, payload []byte) error {
	var deleted videoevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.AuthorID == 0 {
		return consumer.Permanent(errors.New("删除视频事件里没有 authorId"))
	}
	if err := l.users.DecreaseVideoCount(ctx, deleted.AuthorID); err != nil {
		slog.Error("更新视频数量失败", "author_id", deleted.AuthorID, "error", err)
		return err
	}
	return nil
}

func (l *Logic) toInfoRes(item *userrepo.User) *InfoRes {
	return &InfoRes{
		UserID:        item.ID,
		Username:      item.Username,
		Nickname:      item.NickName,
		AvatarURL:     l.uploader.URL(item.AvatarURL),
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
