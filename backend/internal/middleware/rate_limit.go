package middleware

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"

	"feed-system/internal/pkg/errs"
)

// 限流 key 前缀。redis_rate 会在最前面再补 "rate:",
// 最终形如 rate:feed:rate_limit:ip:1.2.3.4
const rateLimitPrefix = "feed:rate_limit:"

// IPRateLimiter 按客户端来源 IP 限流。
//
// 用 RemoteIP 而不是 ClientIP:后者读 X-Forwarded-For,gin 默认信任所有来源,
// 客户端改这个头就是一个全新配额桶,限流会被完全绕过。
// 代价:挂反向代理后所有人共用一个桶,那时要改成「配可信网段 + ClientIP」
func IPRateLimiter(rdb *redis.Client, limit redis_rate.Limit) gin.HandlerFunc {
	if rdb == nil {
		return passThrough
	}
	limiter := redis_rate.NewLimiter(rdb)
	return func(c *gin.Context) {
		if allowOrAbort(c, limiter, rateLimitPrefix+"ip:"+c.RemoteIP(), limit) {
			c.Next()
		}
	}
}

// UserRateLimiter 按登录用户限流。必须挂在 Auth 之后 —— userID 由 Auth 注入,
// 挂错了不报错也不 panic,只等于没限流。
// 与 IPRateLimiter 互补:这个精确到人、不受 NAT 连累,但盖不住匿名接口
func UserRateLimiter(rdb *redis.Client, limit redis_rate.Limit) gin.HandlerFunc {
	if rdb == nil {
		return passThrough
	}
	limiter := redis_rate.NewLimiter(rdb)
	return func(c *gin.Context) {
		userID := UserID(c)
		if userID <= 0 {
			// 挂对位置就不会走到这;走到了基本是路由漏挂 Auth,记一条别静默放过
			slog.ErrorContext(c.Request.Context(), "用户限流取不到 userID,本次跳过(检查路由是否漏挂 Auth)")
			c.Next()
			return
		}
		if allowOrAbort(c, limiter, rateLimitPrefix+"user:"+strconv.FormatInt(userID, 10), limit) {
			c.Next()
		}
	}
}

// passThrough 原样放行。Redis 不可用时用它让限流整体失效
func passThrough(c *gin.Context) {
	c.Next()
}

// allowOrAbort 判断是否放行;超限时写好响应并返回 false。
//
// 读 Allowed 而不是 Remaining:burst 里把桶用光的那一个请求 Remaining 恰好是 0,
// 但它其实是放行的 —— 用 Remaining<=0 会让 PerSecond(1) 变成「全拒」(实测)
func allowOrAbort(c *gin.Context, limiter *redis_rate.Limiter, key string, limit redis_rate.Limit) bool {
	ctx := c.Request.Context()
	res, err := limiter.Allow(ctx, key, limit)
	if err != nil {
		// Redis 故障就放行(fail-open):限流组件不该把整站打死
		slog.ErrorContext(ctx, "限流检查失败,本次放行", "key", key, "err", err)
		return true
	}
	if res.Allowed > 0 {
		return true
	}

	// 库用 -1 表示「不用等」,那种情况不写这个头
	if res.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(int(math.Ceil(res.RetryAfter.Seconds()))))
	}
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"code": errs.ErrTooFrequent.Code,
		"msg":  "请求过于频繁,请稍后再试",
	})
	return false
}
