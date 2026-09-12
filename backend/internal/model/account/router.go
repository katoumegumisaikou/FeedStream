package account

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"feed-system/internal/middleware"
)

// RegisterRouter 把 account 模块的所有路由挂到指定的路由组
// 用法:account.RegisterRouter(r.Group("/api/v1"), h, db, rdb)
//
// Auth 中间件在 internal/middleware 包,只依赖 gorm + redis,不依赖任何业务包,
// 因此 account → middleware 是单向依赖,不会有循环引用
func RegisterRouter(rg *gin.RouterGroup, h *AccountHandler, db *gorm.DB, rdb *redis.Client) {
	// ========== 公开路由(无需登录) ==========
	auth := rg.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/sms-code", h.SendSmsCode)
		auth.PUT("/password", h.ChangePassword) // 走 SMS 验证码流程,无需 JWT
	}

	// ========== 公开路由(查他人资料,无需登录) ==========
	users := rg.Group("/users")
	{
		users.GET("/:id", h.GetProfile)
	}

	// ========== 私有路由(需要登录,Auth 中间件校验 token + version) ==========
	priv := rg.Group("/users", middleware.Auth(db, rdb))
	{
		priv.GET("/me", h.GetMyProfile)
		priv.PUT("/me", h.UpdateProfile)
	}
}