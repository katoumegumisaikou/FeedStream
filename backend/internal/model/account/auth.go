// Package account 中的 Auth 中间件
package account

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"feed-system/internal/pkg/errs"
	"feed-system/internal/pkg/token"
)

const (
	userIDKey      = "userID"
	usersTable     = "users"
	versionCol     = "version"
	versionCacheTTL = 5 * time.Minute
)

// Auth 用户鉴权中间件
// 用法:router.GET("/api/v1/users/me", middleware.Auth(db, rdb), handler.GetMyProfile)
//
// 流程:
//  1. 从 cookie 或 Authorization 头读 access_token
//  2. JWT 签名校验(token.Parse)
//  3. 注入 userID 到 gin.Context
//  4. 查 users 表当前 version(优先 Redis,miss 时回源 DB 并回写)
//  5. 不一致 → 401(改密 / 注销后旧 token 失效)
//  6. 放行
//
// 中间件只依赖 gorm + redis,不依赖任何业务包
func Auth(db *gorm.DB, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := readToken(c)
		if tokenStr == "" {
			abortUnauthorized(c, "未登录")
			return
		}
		claims, err := token.Parse(tokenStr)
		if err != nil {
			abortUnauthorized(c, "token 无效或已过期")
			return
		}
		c.Set(userIDKey, claims.UserID)
		if !checkUserVersion(c, db, rdb, claims.UserID, claims.Version) {
			return
		}
		c.Next()
	}
}

// checkUserVersion 校验 token 中的 version 与 DB 当前 version 是否一致
// 优先读 Redis 缓存;缓存 miss / Redis 不可用时回源 DB,再回写缓存
// 返回 true 表示放行;false 表示已通过 abort 终止请求,调用方直接 return 即可
func checkUserVersion(c *gin.Context, db *gorm.DB, rdb *redis.Client, userID, tokenVersion int64) bool {
	ctx := c.Request.Context()
	cacheKey := userVersionKey(userID)

	// 1. 优先查 Redis
	if rdb != nil {
		cached, err := rdb.Get(ctx, cacheKey).Int64()
		if err == nil {
			if tokenVersion != cached {
				abortUnauthorized(c, "token 已失效,请重新登录")
				return false
			}
			return true
		}
		// redis.Nil 或其他错误:都走 DB,不做中断
	}

	// 2. 回源 DB
	var dbVersion int64
	result := db.WithContext(ctx).
		Table(usersTable).
		Select(versionCol).
		Where("id = ?", userID).
		Take(&dbVersion)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			abortUnauthorized(c, "用户不存在")
			return false
		}
		abortUnauthorized(c, "token 校验失败")
		return false
	}

	// 3. 回写 Redis(只缓存有效用户,TTL 内改密需要同步清缓存才能立即生效)
	if rdb != nil {
		rdb.Set(ctx, cacheKey, dbVersion, versionCacheTTL)
	}

	// 4. 对比
	if tokenVersion != dbVersion {
		abortUnauthorized(c, "token 已失效,请重新登录")
		return false
	}
	return true
}

// userVersionKey 生成用户 version 缓存的 Redis key
func userVersionKey(userID int64) string {
	return fmt.Sprintf("feed:user:version:%d", userID)
}

// UserID 从 gin.Context 取当前登录用户 ID
// 必须在 Auth 中间件之后调用
func UserID(c *gin.Context) int64 {
	v, _ := c.Get(userIDKey)
	id, _ := v.(int64)
	return id
}

// readToken 优先读 cookie,再读 Authorization header
func readToken(c *gin.Context) string {
	if v, err := c.Cookie("access_token"); err == nil && v != "" {
		return v
	}
	if auth := c.GetHeader("Authorization"); hasBearerPrefix(auth) {
		return auth[7:] // len("Bearer ") = 7
	}
	return ""
}

func hasBearerPrefix(s string) bool {
	return len(s) >= 7 && s[:7] == "Bearer "
}

// abortUnauthorized 401 终止请求
func abortUnauthorized(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(401, gin.H{
		"code": errs.ErrUnauthorized.Code,
		"msg":  msg,
	})
}
