package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// newTestRedis 用 miniredis 起一个内存 Redis 并返回客户端。
// 限流脚本用的是 Lua(GCRA),miniredis 能跑
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

// newTestEngine 把待测中间件挂到 /t,后面接一个固定返回 200 的 handler
func newTestEngine(mws ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handlers := make([]gin.HandlerFunc, 0, len(mws)+1)
	handlers = append(handlers, mws...)
	handlers = append(handlers, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"code": 0}) })
	r.GET("/t", handlers...)
	return r
}

// call 发一次请求。remoteAddr 是 TCP 来源地址,forwardedFor 是客户端可伪造的头
func call(r *gin.Engine, remoteAddr, forwardedFor string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestIPRateLimiter(t *testing.T) {
	t.Run("超过上限返回429", func(t *testing.T) {
		// burst = 2:前两次放行,第三次开始拦
		r := newTestEngine(IPRateLimiter(newTestRedis(t), redis_rate.PerSecond(2)))
		const addr = "10.0.0.1:1234"

		assert.Equal(t, http.StatusOK, call(r, addr, "").Code)
		assert.Equal(t, http.StatusOK, call(r, addr, "").Code)

		w := call(r, addr, "")
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.JSONEq(t, `{"code":10006,"msg":"请求过于频繁,请稍后再试"}`, w.Body.String())
		// 必须是实数秒:库用 -1 表示「不用等」,不判正负会写出 Retry-After: 0
		assert.Equal(t, "1", w.Header().Get("Retry-After"))
	})

	t.Run("Redis不可用时放行且不 panic", func(t *testing.T) {
		// main.go 里 Redis 连不上会把 rdb 降级成 nil。若照常调 NewLimiter(nil),
		// 第一次 Allow 就是 nil 解引用 panic,所以这条必须显式短路
		r := newTestEngine(IPRateLimiter(nil, redis_rate.PerSecond(1)))
		assert.Equal(t, http.StatusOK, call(r, "10.0.0.1:1234", "").Code)
	})

	t.Run("不同IP是独立配额", func(t *testing.T) {
		r := newTestEngine(IPRateLimiter(newTestRedis(t), redis_rate.PerSecond(1)))
		assert.Equal(t, http.StatusOK, call(r, "10.0.0.1:1111", "").Code)
		assert.Equal(t, http.StatusOK, call(r, "10.0.0.2:2222", "").Code)
	})

	t.Run("伪造XForwardedFor不能绕过", func(t *testing.T) {
		// 中间件读 RemoteIP 而不是 ClientIP,所以换 XFF 头不该换来新配额。
		// 用 ClientIP 的话这三行会全是 200,限流形同虚设
		r := newTestEngine(IPRateLimiter(newTestRedis(t), redis_rate.PerSecond(1)))

		assert.Equal(t, http.StatusOK, call(r, "10.0.0.1:1234", "1.2.3.1").Code)
		assert.Equal(t, http.StatusTooManyRequests, call(r, "10.0.0.1:1234", "1.2.3.2").Code)
		assert.Equal(t, http.StatusTooManyRequests, call(r, "10.0.0.1:1234", "1.2.3.3").Code)
	})
}

func TestUserRateLimiter(t *testing.T) {
	// asUser 顶替 Auth:真实实现也是把 userID 塞进 gin.Context
	asUser := func(id int64) gin.HandlerFunc {
		return func(c *gin.Context) { c.Set(userIDKey, id) }
	}

	t.Run("按用户限流且各用户配额独立", func(t *testing.T) {
		rdb := newTestRedis(t)

		r7 := newTestEngine(asUser(7), UserRateLimiter(rdb, redis_rate.PerSecond(1)))
		assert.Equal(t, http.StatusOK, call(r7, "10.0.0.1:1234", "").Code)
		assert.Equal(t, http.StatusTooManyRequests, call(r7, "10.0.0.1:1234", "").Code)

		// 同一个 Redis,换用户应该重新拿满配额 —— key 里必须带 userID
		r8 := newTestEngine(asUser(8), UserRateLimiter(rdb, redis_rate.PerSecond(1)))
		assert.Equal(t, http.StatusOK, call(r8, "10.0.0.1:1234", "").Code)
	})

	t.Run("取不到userID时放行而不是拦截", func(t *testing.T) {
		// 挂在 Auth 之前会走到这里。选择放行 + 记日志:路由配错不该变成全站 429
		r := newTestEngine(UserRateLimiter(newTestRedis(t), redis_rate.PerSecond(1)))
		assert.Equal(t, http.StatusOK, call(r, "10.0.0.1:1234", "").Code)
	})

	t.Run("Redis不可用时放行且不 panic", func(t *testing.T) {
		r := newTestEngine(asUser(7), UserRateLimiter(nil, redis_rate.PerSecond(1)))
		assert.Equal(t, http.StatusOK, call(r, "10.0.0.1:1234", "").Code)
	})
}

// TestUserRateLimiter_顺序依赖 锁住「必须排在 Auth 之后」:排错了不报错不 panic,
// 只会静默一条都不限,所以得有测试看着
func TestUserRateLimiter_顺序依赖(t *testing.T) {
	// asAuth 顶替 Auth:真实实现的行为也就是「写 userID + c.Next()」
	asAuth := func(c *gin.Context) { c.Set(userIDKey, int64(7)); c.Next() }

	t.Run("排在Auth之后正常限流", func(t *testing.T) {
		r := newTestEngine(asAuth, UserRateLimiter(newTestRedis(t), redis_rate.PerSecond(1)))
		assert.Equal(t, http.StatusOK, call(r, "10.0.0.1:1234", "").Code)
		assert.Equal(t, http.StatusTooManyRequests, call(r, "10.0.0.1:1234", "").Code)
	})

	t.Run("排在Auth之前等于没挂", func(t *testing.T) {
		r := newTestEngine(UserRateLimiter(newTestRedis(t), redis_rate.PerSecond(1)), asAuth)
		// 两个请求全是 200 —— 拿不到 userID 就走了放行分支
		assert.Equal(t, http.StatusOK, call(r, "10.0.0.1:1234", "").Code)
		assert.Equal(t, http.StatusOK, call(r, "10.0.0.1:1234", "").Code)
	})
}
