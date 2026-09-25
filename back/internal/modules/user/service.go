package user

import (
	"context"
	"errors"
	"mime/multipart"
	"strings"

	"gorm.io/gorm"

	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/pkg/hash_password"
	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/upload"
)

type Service struct {
	userRepo  *UserRepo
	videoRepo *VideoRepo
	auth      *auth.Service
	uploader  *upload.Uploader
}

func NewService(userRepo *UserRepo, videoRepo *VideoRepo, authService *auth.Service, uploader *upload.Uploader) *Service {
	return &Service{
		userRepo:  userRepo,
		videoRepo: videoRepo,
		auth:      authService,
		uploader:  uploader,
	}
}

func (s *Service) Register(ctx context.Context, registerReq req.RegisterReq) (uint64, error) {
	if registerReq.Password != registerReq.RePassword {
		return 0, httpx.New(httpx.CodeBadRequest, "两次输入的密码不一致")
	}
	if err := checkValidUsernameAndPassword(registerReq.Username, registerReq.Password); err != nil {
		return 0, err
	}
	hashPassword, err := hash_password.HashPassword(registerReq.Password)
	if err != nil {
		return 0, err
	}

	user, err := s.userRepo.CreateUser(registerReq.Username, hashPassword, constants.DefaultAvatar)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return 0, httpx.New(httpx.CodeConflict, "用户名已存在")
		}
		return 0, err
	}
	return user.ID, nil
}

func (s *Service) Login(ctx context.Context, username string, password string) (string, error) {
	if err := checkValidUsernameAndPassword(username, password); err != nil {
		return "", err
	}
	user, err := s.userRepo.GetUserByUserNameAndPassword(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", httpx.New(httpx.CodeCredential, "用户名或密码错误")
		}
		return "", err
	}
	if !hash_password.CheckPassword(password, user.Password) {
		return "", httpx.New(httpx.CodeCredential, "用户名或密码错误")
	}
	return s.auth.GenerateToken(ctx, user.ID, user.NickName)
}

func (s *Service) GetUserInfo(userID uint64) (*res.UserInfoRes, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.New(httpx.CodeNotFound, "用户不存在")
		}
		return nil, err
	}
	videoCount, err := s.videoRepo.CountByAuthorID(userID)
	if err != nil {
		return nil, err
	}
	return &res.UserInfoRes{
		UserID:        user.ID,
		Username:      user.Username,
		Nickname:      user.NickName,
		AvatarURL:     s.uploader.URL(user.AvatarURL),
		FollowCount:   user.FollowCount,
		FollowerCount: user.FollowerCount,
		VideoCount:    videoCount,
	}, nil
}

func (s *Service) UpdateProfile(userID uint64, nickname string, avatar *multipart.FileHeader) (*res.UserInfoRes, error) {
	user, err := s.userRepo.GetUserByID(userID)
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
		newAvatarPath, err = s.uploader.Save(avatar, upload.Avatar)
		if err != nil {
			return nil, err
		}
		updates["avatar_url"] = newAvatarPath
	}

	if err = s.userRepo.UpdateProfile(userID, updates); err != nil {
		if newAvatarPath != "" {
			_ = s.uploader.Delete(upload.Avatar, newAvatarPath)
		}
		return nil, err
	}

	if nickname != "" && nickname != user.NickName {
		if err = s.videoRepo.UpdateAuthorNameByAuthorID(userID, nickname); err != nil {
			return nil, err
		}
	}

	if newAvatarPath != "" && user.AvatarURL != "" && user.AvatarURL != constants.DefaultAvatar {
		_ = s.uploader.Delete(upload.Avatar, user.AvatarURL)
	}

	return s.GetUserInfo(userID)
}

func (s *Service) Logout(ctx context.Context, userID uint64) error {
	return s.auth.Revoke(ctx, userID)
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
