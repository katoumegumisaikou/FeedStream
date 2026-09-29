package feed

import (
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"feed-system/internal/middleware"
)

var latestFeedIPLimit = redis_rate.PerMinute(120)

// RegisterRouter 注册公开的视频流路由。
// 为兼容现有客户端,最新视频仍使用 GET /videos/latest。
func RegisterRouter(rg *gin.RouterGroup, h *FeedHandler, db *gorm.DB, rdb *redis.Client) {
	pub := rg.Group("/videos",
		middleware.IPRateLimiter(rdb, latestFeedIPLimit),
		middleware.Auth(db, rdb),
	)
	pub.GET("/latest", h.ListLatest)
}
