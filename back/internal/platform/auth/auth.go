package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"simple_tiktok/internal/platform/httpx"
)

const (
	// TokenKey Redis 里保存 token 的键格式
	TokenKey = "jwt_token:%d"
	// CtxUserID 与 CtxUserNickName 是写入 gin 上下文使用的键
	CtxUserID       = "userId"
	CtxUserNickName = "userNickName"

	defaultExpireHours = 72
)

type Claims struct {
	UserID       uint64 `json:"user_id"`
	UserNickName string `json:"user_nick_name"`
	jwt.RegisteredClaims
}

// Service 负责签发与校验登录凭证
type Service struct {
	secret      string
	expireHours int64
	redisClient *redis.Client
}

func New(secret string, expireHours int64, redisClient *redis.Client) (*Service, error) {
	if secret == "" {
		return nil, fmt.Errorf("jwt secret 为空")
	}
	if expireHours <= 0 {
		expireHours = defaultExpireHours
	}
	return &Service{
		secret:      secret,
		expireHours: expireHours,
		redisClient: redisClient,
	}, nil
}

// GenerateToken 签发 token 并写入 Redis
func (s *Service) GenerateToken(ctx context.Context, userID uint64, nickName string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		UserNickName: nickName,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenTTL())),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
	if err != nil {
		return "", err
	}
	if err := s.redisClient.Set(ctx, s.tokenKey(userID), token, s.tokenTTL()).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// Revoke 删除 Redis 里保存的 token
func (s *Service) Revoke(ctx context.Context, userID uint64) error {
	return s.redisClient.Del(ctx, s.tokenKey(userID)).Err()
}

// Middleware 校验请求头里的 token，把用户身份写入 gin 上下文，并顺带刷新有效期
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "请先登录"))
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
			c.Abort()
			return
		}

		claims, err := s.parseToken(parts[1])
		if err != nil {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
			c.Abort()
			return
		}

		ctx := c.Request.Context()
		refreshed, err := s.redisClient.Expire(ctx, s.tokenKey(claims.UserID), s.tokenTTL()).Result()
		if err != nil {
			httpx.Fail(c, err)
			c.Abort()
			return
		}
		if !refreshed {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录已失效，请重新登录"))
			c.Abort()
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUserNickName, claims.UserNickName)
		c.Next()
	}
}

// UserID 从 gin 上下文取出当前登录用户 ID，取不到时返回 0
func UserID(c *gin.Context) uint64 {
	value, exists := c.Get(CtxUserID)
	if !exists {
		return 0
	}
	userID, _ := value.(uint64)
	return userID
}

// NickName 从 gin 上下文取出当前登录用户昵称
func NickName(c *gin.Context) string {
	value, exists := c.Get(CtxUserNickName)
	if !exists {
		return ""
	}
	nickName, _ := value.(string)
	return nickName
}

func (s *Service) parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("签名方式不符合预期")
		}
		return []byte(s.secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("token 无效")
	}
	return claims, nil
}

func (s *Service) tokenKey(userID uint64) string {
	return fmt.Sprintf(TokenKey, userID)
}

func (s *Service) tokenTTL() time.Duration {
	return time.Duration(s.expireHours) * time.Hour
}
