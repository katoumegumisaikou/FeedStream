package video

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"feed-system/internal/middleware"
)

// RegisterRouter 把 video 模块的所有路由挂到指定的路由组
// 用法:video.RegisterRouter(r.Group("/api/v1"), h, db, rdb)
//
// Auth 中间件在 internal/middleware 包,只依赖 gorm + redis,不依赖任何业务包,
// 因此 video → middleware 是单向依赖,不会有循环引用
func RegisterRouter(rg *gin.RouterGroup, h *VideoHandler, db *gorm.DB, rdb *redis.Client) {
	// 上传相关的接口全部强制登录:都是写操作,而且 UploadChunk / Complete
	// 要拿 userID 做会话归属校验 —— 只有知道「你是谁」,
	// 才能判断「这个 upload_id 是不是你的」
	//
	// 用 /videos/chunk 前缀而不是散在 /videos 下:gin 允许静态段与
	// 通配段共存,所以以后加 GET /videos/:id 不会和这里冲突
	priv := rg.Group("/videos", middleware.SetSensitive(), middleware.Auth(db, rdb))
	{
		priv.POST("/chunk/init", h.InitChunkUpload)         // 初始化上传,拿 upload_id
		priv.POST("/chunk", h.UploadChunk)                  // 上传一个分片
		priv.POST("/chunk/complete", h.CompleteChunkUpload) // 合并分片,生成草稿视频
	}
}
