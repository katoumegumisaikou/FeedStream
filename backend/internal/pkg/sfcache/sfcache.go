// Package sfcache 提供「并发去重的缓存加载」,解决缓存击穿:
// 同一 key 的并发未命中只会真正回源一次,其余共享结果。
//
// 去重域是进程级的,key 必须全进程唯一(各业务自带前缀)。
// 值的编解码由本包负责,调用方拿到的就是自己的类型 T。
package sfcache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"
)

// ErrMiss 缓存未命中,与真正的故障区分开:前者回源,后者由调用方决定降级
var ErrMiss = errors.New("sfcache: cache miss")

// loadTimeout 回源超时。回源用的 ctx 剥掉了调用方 ctx(见 Load),
// 不会随调用方断开而取消,所以必须自己兜一个 deadline
const loadTimeout = 3 * time.Second

// sf 进程级去重域,所有 Load[T] 共用(泛型不会复制包级变量)。
// 因此 key 必须全进程唯一,各业务各带前缀;撞 key 会让一方拿到另一方的结果
var sf singleflight.Group

// Cache 最小缓存契约:值一律按字符串存取,编解码由本包负责
type Cache interface {
	// Get 未命中必须返回 ErrMiss
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// Load 取 key 对应的值:
//
//  1. 先查缓存,命中直接返回(此步在 singleflight 之外,避免把正常读也串行化)
//  2. 未命中则回源,同一 key 的并发调用只回源一次,其余共享结果
//  3. 回源成功且 cache 非 nil 时回写缓存
//
// T 从 load 的返回类型推导。⚠️ T 的字段必须导出:JSON 看不见未导出字段且不报错,
// 会静默编成 {} 再解出全零值,变成没有报错的错数据。类型名本身可以小写。
//
// cache 传 nil 表示跳过缓存,只要并发去重。
// 回源失败也在并发调用间共享(避免失败时挨个重试),但失败不写缓存。
func Load[T any](
	ctx context.Context,
	cache Cache,
	key string,
	ttl time.Duration,
	load func(context.Context) (T, error),
) (T, error) {
	var zero T

	if cache != nil {
		raw, err := cache.Get(ctx, key)
		switch {
		case err == nil:
			// 命中:存的是 JSON,直接解进 T,不经过 any
			var out T
			if json.Unmarshal([]byte(raw), &out) == nil {
				return out, nil
			}
			// 解不开(值被写脏或 key 被另一个 T 复用):当未命中回源自愈,
			// 报错会把调用方一直挡到 TTL 过期
		case !errors.Is(err, ErrMiss):
			// 缓存故障不当成未命中,降级策略由调用方定
			return zero, fmt.Errorf("读取缓存失败: %w", err)
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
		if cache != nil {
			// 编不出来就跳过缓存:缓存只是加速,不是真相来源
			if b, mErr := json.Marshal(val); mErr == nil {
				_ = cache.Set(loadCtx, key, string(b), ttl)
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
