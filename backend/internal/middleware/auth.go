// Package middleware 存放 gin 中间件。
// 只依赖 gorm / redis / pkg 层工具,不依赖业务包,避免与 model 层循环引用。
package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"feed-system/internal/pkg/errs"
	"feed-system/internal/pkg/sfcache"
	"feed-system/internal/pkg/token"
)

const (
	userIDKey       = "user_id"
	usersTable      = "users"
	versionCol      = "version"
	versionCacheTTL = 5 * time.Minute
)

// SetSensitive 将当前路由标记为强制登录，需与 Auth 一起使用。
func SetSensitive() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("sensitive", true)
	}
}

// Auth 用户鉴权中间件,从 cookie 或 Authorization 头取 token,校验签名后注入 userID,
// 再比对 users.version(见 checkUserVersion);sensitive 路由无 token 直接 401。
// 用法:router.GET("/api/v1/users/me", middleware.Auth(db, rdb), handler.GetMyProfile)
func Auth(db *gorm.DB, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := readToken(c)
		sensitive := c.GetBool("sensitive")
		if sensitive && tokenStr == "" {
			abortUnauthorized(c, "未登录")
			return
		} else if !sensitive && tokenStr == "" {
			c.Next()
			return
		}
		claims, err := token.Parse(tokenStr)
		if err != nil {
			abortUnauthorized(c, "token 无效或已过期")
			return
		}

		if !checkUserVersion(c, db, rdb, claims.UserID, claims.Version) {
			return
		}
		c.Set(userIDKey, claims.UserID)
		c.Next()
	}
}

// versionCache 把 Redis 适配成 sfcache.Cache。
// Redis 存的本来就是字符串,所以这里不需要额外编解码
//
// 读失败一律当作未命中(含 redis.Nil、值不是数字、真故障):
// Redis 挂了不该让鉴权跟着挂,回源 DB 即可
type versionCache struct{ rdb *redis.Client }

func (c versionCache) Get(ctx context.Context, key string) (string, error) {
	if c.rdb == nil {
		return "", sfcache.ErrMiss
	}
	raw, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", sfcache.ErrMiss
	}
	// 顺手验一下是数字:值被外部写脏时当未命中、回源自愈,
	// 否则用户会被一个非法值挡在门外直到 TTL 过期
	if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
		return "", sfcache.ErrMiss
	}
	return raw, nil
}

func (c versionCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// checkUserVersion 比对 token 中的 version 与 DB 当前 version,不一致则 401 ——
// 改密 / 注销会改 version,使旧 token 失效。
// 缓存优先,miss 时回源 DB 并回写;同一用户的并发回源会被合并成一次。
// 返回 false 表示已 abort,调用方直接 return。
//
// 走包级 sfcache.Load:它用的是进程级唯一 Group,所以并发去重不依赖调用方
// 维护什么生命周期。key 自带 feed:user:version: 前缀,不会和别的业务撞
func checkUserVersion(c *gin.Context, db *gorm.DB, rdb *redis.Client, userID, tokenVersion int64) bool {
	ctx := c.Request.Context()

	raw, err := sfcache.Load(ctx, versionCache{rdb: rdb}, userVersionKey(userID), versionCacheTTL,
		func(loadCtx context.Context) (string, error) {
			var v int64
			if err := db.WithContext(loadCtx).
				Table(usersTable).
				Select(versionCol).
				Where("id = ?", userID).
				Take(&v).Error; err != nil {
				return "", err
			}
			return strconv.FormatInt(v, 10), nil
		})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			abortUnauthorized(c, "用户不存在")
			return false
		}
		abortUnauthorized(c, "token 校验失败")
		return false
	}

	dbVersion, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		// 适配器已经验过是数字,走到这说明哪里不对 —— 记一条,别静默成 401
		slog.ErrorContext(ctx, "version 缓存返回了非数字", "user_id", userID, "value", raw)
		abortUnauthorized(c, "token 校验失败")
		return false
	}

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
