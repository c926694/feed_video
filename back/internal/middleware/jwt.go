package middleware

import (
	"context"
	"fmt"
	"simple_tiktok/internal/initialize"
	"simple_tiktok/internal/pkg/jwt"
	"simple_tiktok/internal/platform/httpx"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type JWT struct {
	redisClient *redis.Client
}

const (
	TokenKey     = "jwt_token:%d"
	UserCtx      = "userId"
	UserNickName = "userNickName"
)

func JWTAuth(redisClient *redis.Client) gin.HandlerFunc {
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

		claims, err := jwt.ParseToken(parts[1])
		if err != nil {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
			c.Abort()
			return
		}
		userId := claims.UserID
		nickName := claims.UserNickName
		key := fmt.Sprintf(TokenKey, userId)
		ctx := context.Background()
		token, err := redisClient.Get(ctx, key).Result()
		if err != nil {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录已失效，请重新登录"))
			c.Abort()
			return
		}
		//刷新token
		expire := initialize.AppConfig.JWT.ExpireHours
		if _, err = redisClient.Set(ctx, key, token, time.Duration(expire)*time.Hour).Result(); err != nil {
			httpx.Fail(c, err)
			c.Abort()
			return
		}
		c.Set(UserCtx, userId)
		c.Set(UserNickName, nickName)
		c.Next()
	}
}
