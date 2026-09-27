// Package sfcache 提供「并发去重的缓存加载」,解决缓存击穿:
// 同一 key 的并发未命中只会真正回源一次,其余共享结果。
//
// 去重域是进程级的,key 必须全进程唯一(各业务自带前缀)。
// 值的编解码由本包负责,调用方拿到的就是自己的类型 T。
//
// 缓存后端就是 Redis。读写出错一律降级(读当未命中、写直接跳过)并记一条日志 ——
// 本包服务的是鉴权和文件路由,挡在缓存这一层等于把一次缓存故障放大成全站故障。
package sfcache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// loadTimeout 回源超时。回源用的 ctx 剥掉了调用方 ctx(见 Load),
// 不会随调用方断开而取消,所以必须自己兜一个 deadline
const loadTimeout = 3 * time.Second

// sf 进程级去重域,所有 Load[T] 共用(泛型不会复制包级变量)。
// 因此 key 必须全进程唯一,各业务各带前缀;撞 key 会让一方拿到另一方的结果
var sf singleflight.Group

// Load 取 key 对应的值:
//
//  1. 先查缓存,命中直接返回(此步在 singleflight 之外,避免把正常读也串行化)
//  2. 未命中则回源,同一 key 的并发调用只回源一次,其余共享结果
//  3. 回源成功且 rdb 非 nil 时回写缓存
//
// T 从 load 的返回类型推导。⚠️ T 的字段必须导出:JSON 看不见未导出字段且不报错,
// 会静默编成 {} 再解出全零值,变成没有报错的错数据。类型名本身可以小写。
//
// rdb 传 nil 表示跳过缓存,只要并发去重 —— main.go 的 mustRedisClient 连不上时
// 就返回 nil,生产环境本就会走到这条路径。
// 回源失败也在并发调用间共享(避免失败时挨个重试),但失败不写缓存。
func Load[T any](
	ctx context.Context,
	rdb *redis.Client,
	key string,
	ttl time.Duration,
	load func(context.Context) (T, error),
) (T, error) {
	var zero T

	if rdb != nil {
		raw, err := rdb.Get(ctx, key).Result()
		switch {
		case err == nil:
			// 命中:存的是 JSON,直接解进 T,不经过 any
			var out T
			if json.Unmarshal([]byte(raw), &out) == nil {
				return out, nil
			}
			// 解不开(值被写脏或 key 被另一个 T 复用):当未命中回源自愈,
			// 报错会把调用方一直挡到 TTL 过期
		case !errors.Is(err, redis.Nil):
			// 真故障也降级回源 —— 缓存这一层没有别的选择。但不静默:
			// Redis 挂一整天却没人知道,外在表现只是「缓存全部失效」
			slog.ErrorContext(ctx, "读缓存失败,降级回源", "key", key, "err", err)
		}
	}

	v, err, _ := sf.Do(key, func() (any, error) {
		// 剥掉发起者的 ctx:leader 断开不该连累跟随者,代价是丢了 deadline,
		// 所以上面自己兜了 loadTimeout
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), loadTimeout)
		defer cancel()

		val, err := load(loadCtx)
		if err != nil {
			return nil, err
		}
		if rdb != nil {
			// 编不出来、或写不进去,都只记日志:缓存是加速层,不是真相来源
			if b, mErr := json.Marshal(val); mErr == nil {
				if wErr := rdb.Set(loadCtx, key, string(b), ttl).Err(); wErr != nil {
					slog.ErrorContext(loadCtx, "回写缓存失败", "key", key, "err", wErr)
				}
			}
		}
		return val, nil
	})
	if err != nil {
		return zero, err
	}

	// Do 返回 any,断言只能在这里做 —— 只有本函数分得清值来自缓存(JSON 解析)
	// 还是 singleflight(leader 塞的 Go 值)。
	// 用 comma-ok 而非直接断言:撞 key 时存入的可能是别的 T,那是编码错误,不该崩进程
	out, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("sfcache: key %q 被复用了不同类型:存入的是 %T", key, v)
	}
	return out, nil
}
