package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"simple_tiktok/internal/platform/httpx"
)

const (
	// RefreshKey 是 refresh token 在 Redis 里的键格式，值是用户 ID
	RefreshKey = "refresh:%s"
	// CtxUserID 是写入 gin 上下文使用的键
	CtxUserID = "userId"

	defaultAccessMinutes = 15
	defaultRefreshHours  = 336
	refreshTokenBytes    = 32
)

// AccessClaims 是 access token 的内容，只用标准字段
type AccessClaims struct {
	jwt.RegisteredClaims
}

// TokenPair 登录与刷新返回给前端的一对令牌
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// Service 负责签发、校验、撤销登录凭证。
// access token 是无状态 JWT，只验签与校验有效期；refresh token 是随机串，
// 服务端按原文存一条记录，刷新时整体轮换，登出时删除。
type Service struct {
	secret      string
	accessTTL   time.Duration
	refreshTTL  time.Duration
	redisClient *redis.Client
}

func New(secret string, accessMinutes int64, refreshHours int64, redisClient *redis.Client) (*Service, error) {
	if secret == "" {
		return nil, fmt.Errorf("jwt secret 为空")
	}
	if accessMinutes <= 0 {
		accessMinutes = defaultAccessMinutes
	}
	if refreshHours <= 0 {
		refreshHours = defaultRefreshHours
	}
	return &Service{
		secret:      secret,
		accessTTL:   time.Duration(accessMinutes) * time.Minute,
		refreshTTL:  time.Duration(refreshHours) * time.Hour,
		redisClient: redisClient,
	}, nil
}

// Issue 登录：生成 refresh token 并写进 Redis，签发 access token
func (s *Service) Issue(ctx context.Context, userID uint64) (TokenPair, error) {
	refreshToken, err := randomToken(refreshTokenBytes)
	if err != nil {
		return TokenPair{}, err
	}
	if err = s.redisClient.Set(ctx, refreshKey(refreshToken), strconv.FormatUint(userID, 10), s.refreshTTL).Err(); err != nil {
		return TokenPair{}, err
	}
	return s.signAccess(userID, refreshToken), nil
}

// Refresh 用 refresh token 换一对新令牌。旧 refresh 立即失效，新 refresh 接替。
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	invalid := httpx.New(httpx.CodeUnauthorized, "登录已失效，请重新登录")
	userIDStr, err := s.redisClient.Get(ctx, refreshKey(refreshToken)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return TokenPair{}, invalid
		}
		return TokenPair{}, err
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return TokenPair{}, fmt.Errorf("解析 refresh 记录里的用户 ID 失败: %w", err)
	}

	newRefresh, err := randomToken(refreshTokenBytes)
	if err != nil {
		return TokenPair{}, err
	}
	pipe := s.redisClient.TxPipeline()
	pipe.Del(ctx, refreshKey(refreshToken))
	pipe.Set(ctx, refreshKey(newRefresh), userIDStr, s.refreshTTL)
	if _, err = pipe.Exec(ctx); err != nil {
		return TokenPair{}, err
	}
	return s.signAccess(userID, newRefresh), nil
}

// Revoke 登出：删除这条 refresh 记录，之后再用它刷新会失败
func (s *Service) Revoke(ctx context.Context, refreshToken string) error {
	return s.redisClient.Del(ctx, refreshKey(refreshToken)).Err()
}

// Middleware 校验 access token，只验签与校验有效期，不查存储
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "请先登录"))
			c.Abort()
			return
		}
		claims, err := s.parseAccess(token)
		if err != nil {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
			c.Abort()
			return
		}
		userID, err := strconv.ParseUint(claims.Subject, 10, 64)
		if err != nil {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
			c.Abort()
			return
		}
		c.Set(CtxUserID, userID)
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

func (s *Service) signAccess(userID uint64, refreshToken string) TokenPair {
	now := time.Now()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
	if err != nil {
		return TokenPair{}
	}
	return TokenPair{
		AccessToken:  token,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}
}

func (s *Service) parseAccess(tokenString string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("签名方式不符合预期")
		}
		return []byte(s.secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token 无效")
	}
	return claims, nil
}

func bearerToken(c *gin.Context) (string, bool) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), true
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func refreshKey(refreshToken string) string {
	return fmt.Sprintf(RefreshKey, refreshToken)
}
