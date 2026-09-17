package video

import (
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"feed-system/internal/middleware"
)

// 限流阈值。分片上传 1 GiB = 205 个请求,阈值必须让一次正常上传跑得完
var (
	uploadUserLimit = redis_rate.PerSecond(10)
	uploadIPLimit   = redis_rate.PerSecond(30)
)

// RegisterRouter 把 video 模块的所有路由挂到指定的路由组
// 用法:video.RegisterRouter(r.Group("/api/v1"), h, db, rdb)
//
// Auth 中间件在 internal/middleware 包,只依赖 gorm + redis,不依赖任何业务包,
// 因此 video → middleware 是单向依赖,不会有循环引用
func RegisterRouter(rg *gin.RouterGroup, h *VideoHandler, db *gorm.DB, rdb *redis.Client) {
	// 全部强制登录:UploadChunk / Complete 要拿 userID 校验会话归属
	//
	// 用 /videos/chunk 前缀而不是散在 /videos 下:gin 允许静态段与通配段共存,
	// 以后加 GET /videos/:id 不会冲突
	//
	// 限流顺序不能乱:IP 在最前,UserRateLimiter 依赖 Auth 注入的 userID
	priv := rg.Group("/videos",
		middleware.SetSensitive(),
		middleware.IPRateLimiter(rdb, uploadIPLimit),
		middleware.Auth(db, rdb),
		middleware.UserRateLimiter(rdb, uploadUserLimit),
	)
	{
		priv.POST("/chunk/init", h.InitChunkUpload)         // 初始化上传,拿 upload_id
		priv.POST("/chunk", h.UploadChunk)                  // 上传一个分片
		priv.POST("/chunk/complete", h.CompleteChunkUpload) // 合并分片,生成草稿视频
	}
}
