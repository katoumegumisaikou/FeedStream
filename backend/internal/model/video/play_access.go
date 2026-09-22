package video

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"feed-system/internal/middleware"
	"feed-system/internal/pkg/sfcache"
)

// playAccessCacheTTL 可见性判定的缓存时长。
//
// 播放器一次观看要发几十个 Range 请求,每个都查一次库会放大几十倍,所以挡一层缓存。
// 代价:草稿→发布、或下架之后,最长 TTL 内文件访问还是旧判定
const playAccessCacheTTL = 5 * time.Minute

// playTarget 反查出的可见性判定所需字段。Found=false 表示库里没有这个 play_url。
//
// 字段必须导出:它要经 sfcache 的 JSON 编解码,而 JSON 看不见未导出字段 ——
// 拿不到不会有任何报错,只会静默编成 {},命中缓存时解出全零值把判定带偏
type playTarget struct {
	AuthorID int64 `json:"author_id"`
	Status   int8  `json:"status"`
	Found    bool  `json:"found"`
}

// PlayAccess 是公开文件路由 /videos/{authorID}/{file} 的可见性中间件。
//
// 裸的静态挂载不认识「视频状态」,只要文件在盘上谁来都发 —— 草稿和已下架的
// 视频文件,谁知道路径谁就能下。这层按 play_url 反查视频把判定补上。
//
// 不通过一律 404:403 等于承认「这个文件存在」,草稿的边界就被探出来了
// (与 GetVideoDetail 同一套理由)。404 不带 body,与 http.FileServer 找不到文件时一致
//
// 必须挂在 middleware.Auth 之后 —— 它靠 userID 判「是不是作者」
func PlayAccess(repo VideoRepository, rdb *redis.Client) gin.HandlerFunc {
	cache := playAccessCache{rdb: rdb}

	return func(c *gin.Context) {
		// 组的 URL 前缀 + 通配段正好还原出入库时的 play_url,反查才能走索引
		playURL := VideoURLPrefix + c.Param("filepath")

		target, err := sfcache.Load(c.Request.Context(), cache, playURLKey(playURL), playAccessCacheTTL,
			func(loadCtx context.Context) (playTarget, error) {
				v, err := repo.FindVideoByPlayURL(loadCtx, playURL)
				if errors.Is(err, ErrNotFound) {
					// 「没有这个视频」也当正常结果缓存:失效的旧链接会被反复请求,
					// 不缓存就是每次都打库。真故障才返回 error —— 那样不会被缓存,下次重试
					return playTarget{}, nil
				}
				if err != nil {
					return playTarget{}, err
				}
				return playTarget{AuthorID: v.AuthorID, Status: v.Status, Found: true}, nil
			})
		if err != nil {
			// 判定不了就不放行:失败要么是库挂了、要么是 key 撞了别的类型,
			// 放行等于退回「谁知道路径谁能下」的裸静态行为
			slog.ErrorContext(c.Request.Context(), "播放可见性判定失败", "play_url", playURL, "err", err)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		if !target.Found || !visibleTo(target.Status, target.AuthorID, middleware.UserID(c)) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Next()
	}
}

// playURLKey 缓存 key。前缀必须全局唯一:sfcache 全进程共用一个去重域,
// 撞 key 会把别的业务的结果发给本业务
func playURLKey(playURL string) string {
	return "video:play_url:" + playURL
}

// playAccessCache 把 Redis 适配成 sfcache.Cache。Redis 存的本来就是字符串,
// 这里不需要任何编解码 —— 那是 sfcache 的事
type playAccessCache struct{ rdb *redis.Client }

// Get 一律把读失败当未命中:Redis 挂了不该让文件服务跟着挂,回源 DB 即可
func (c playAccessCache) Get(ctx context.Context, key string) (string, error) {
	if c.rdb == nil {
		return "", sfcache.ErrMiss
	}
	raw, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", sfcache.ErrMiss
	}
	return raw, nil
}

func (c playAccessCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Set(ctx, key, value, ttl).Err()
}
