package user

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/initialize"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/pkg/hash_password"
	"simple_tiktok/internal/pkg/jwt"
	"simple_tiktok/internal/pkg/upload"
	"simple_tiktok/internal/pkg/util"
	"simple_tiktok/internal/platform/httpx"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Service struct {
	userRepo  *UserRepo
	videoRepo *VideoRepo
	userRedis *redis.Client
}

func NewService(userRepo *UserRepo, videoRepo *VideoRepo, redisClient *redis.Client) *Service {
	return &Service{
		userRepo:  userRepo,
		videoRepo: videoRepo,
		userRedis: redisClient,
	}
}

func (s *Service) Register(ctx context.Context, registerReq req.RegisterReq) (uint64, error) {
	if registerReq.Password != registerReq.RePassword {
		return 0, httpx.New(httpx.CodeBadRequest, "两次输入的密码不一致")
	}
	err := checkValidUsernameAndPassword(registerReq.Username, registerReq.Password)
	if err != nil {
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

func (s *Service) Login(username string, password string) (string, error) {
	err := checkValidUsernameAndPassword(username, password)
	if err != nil {
		return "", err
	}
	user, err := s.userRepo.GetUserByUserNameAndPassword(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", httpx.New(httpx.CodeCredential, "用户名或密码错误")
		}
		return "", err
	}
	right := hash_password.CheckPassword(password, user.Password)
	if !right {
		return "", httpx.New(httpx.CodeCredential, "用户名或密码错误")
	}
	token, err := jwt.GenerateToken(user.ID, user.NickName)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf(middleware.TokenKey, user.ID)
	expire := initialize.AppConfig.JWT.ExpireHours
	ctx := context.Background()
	_, err = s.userRedis.Set(ctx, key, token, time.Duration(expire)*time.Hour).Result()
	if err != nil {
		return "", err
	}
	return token, nil
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
		AvatarURL:     util.EnsureHTTPPath(user.AvatarURL),
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
		newAvatarPath, err = upload.UploadFile(avatar, upload.Avatar)
		if err != nil {
			return nil, err
		}
		updates["avatar_url"] = newAvatarPath
	}

	if err = s.userRepo.UpdateProfile(userID, updates); err != nil {
		if newAvatarPath != "" {
			_ = upload.Delete(upload.Avatar, newAvatarPath)
		}
		return nil, err
	}

	if nickname != "" && nickname != user.NickName {
		if err = s.videoRepo.UpdateAuthorNameByAuthorID(userID, nickname); err != nil {
			return nil, err
		}
	}

	if newAvatarPath != "" && user.AvatarURL != "" && user.AvatarURL != constants.DefaultAvatar {
		_ = upload.Delete(upload.Avatar, user.AvatarURL)
	}

	return s.GetUserInfo(userID)
}

func (s *Service) Logout(userID uint64) error {
	_, err := s.userRedis.Del(context.Background(), fmt.Sprintf(middleware.TokenKey, userID)).Result()
	if err != nil {
		return err
	}
	return nil
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
