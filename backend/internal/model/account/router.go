package account

import (
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"feed-system/internal/middleware"
)

// 限流阈值。放模块里而不是 middleware 包:阈值是业务策略,各模块可以不同
var (
	authIPLimit   = redis_rate.PerMinute(30) // 匿名认证接口,按 IP
	publicIPLimit = redis_rate.PerMinute(60) // 公开读接口(查他人资料),按 IP 防爬

	privUserLimit = redis_rate.PerSecond(10) // 已登录接口,按用户:精确到人,可以紧
	privIPLimit   = redis_rate.PerSecond(30) // 已登录接口,按 IP 兜底:要比用户那份宽,NAT 出口后是好多人
)

// RegisterRouter 把 account 模块的所有路由挂到指定的路由组
// 用法:account.RegisterRouter(r.Group("/api/v1"), h, db, rdb)
//
// Auth 中间件在 internal/middleware 包,只依赖 gorm + redis,不依赖任何业务包,
// 因此 account → middleware 是单向依赖,不会有循环引用
func RegisterRouter(rg *gin.RouterGroup, h *AccountHandler, db *gorm.DB, rdb *redis.Client) {
	// ========== 公开路由(无需登录) ==========
	// 匿名可调,是撞库 / 刷短信的主要入口:只能靠 IP 限流(拿不到 userID)
	auth := rg.Group("/auth", middleware.IPRateLimiter(rdb, authIPLimit))
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/sms-code", h.SendSmsCode)
		auth.PUT("/password", h.ChangePassword) // 走 SMS 验证码流程,无需 JWT
	}

	// ========== 公开路由(查他人资料,无需登录) ==========
	// 游客点开作者主页不该被要求登录,安全性由 DTO 兜底:GetProfile 只返回
	// PublicUserResp,不含手机号 / 邮箱 / 最近登录时间。
	// 限流是防爬 —— 按 ID 遍历就能拖库
	users := rg.Group("/users", middleware.IPRateLimiter(rdb, publicIPLimit))
	{
		users.GET("/:id", h.GetProfile)
	}

	// ========== 私有路由(需要登录,Auth 中间件校验 token + version) ==========
	//
	// 顺序不能乱:IP 在最前(洪水在 JWT 解析前就 429),UserRateLimiter 依赖 Auth 注入的 userID。
	// 两层都挂:用户限流精确到人,IP 兜住「同一账号被多机同时刷」
	priv := rg.Group("/users",
		middleware.SetSensitive(),
		middleware.IPRateLimiter(rdb, privIPLimit),
		middleware.Auth(db, rdb),
		middleware.UserRateLimiter(rdb, privUserLimit),
	)
	{
		priv.GET("/me", h.GetMyProfile)
		priv.PUT("/me", h.UpdateProfile)
		priv.POST("/me/avatar", h.UploadAvatar)
	}
}
