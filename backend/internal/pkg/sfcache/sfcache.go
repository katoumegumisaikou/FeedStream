// Package sfcache 提供「并发去重的缓存加载」。
//
// 它解决的是缓存击穿:N 个并发请求同时发现缓存未命中,于是同时回源,
// 把后端打穿。这里让同一 key 的并发加载只真正执行一次,其余共享那一份结果。
//
// 包只有一个入口 Load,去重域是进程级的:调用方拿不到、也不需要持有任何句柄,
// 因此「在调用点临时建一个去重域」这类把去重悄悄关掉的错误做不到。
package sfcache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"
)

// ErrMiss 缓存未命中。Cache 实现用它表示「没有这个 key」,
// 与真正的故障区分开 —— 前者该回源,后者由调用方决定怎么降级
var ErrMiss = errors.New("sfcache: cache miss")

// loadTimeout 回源的超时。
//
// 回源用的是剥掉调用方 ctx 的 context(见 Load),这样 leader 的客户端断开
// 不会连累跟随者;代价是丢了 deadline,所以必须自己兜一个
const loadTimeout = 3 * time.Second

// sf 是进程级去重域。
//
// 因为全进程共用一个,**key 必须在整个进程里唯一**,各业务各带前缀
// (如 feed:user:version:1 与 video:7)。两个业务撞了同一个 key,
// singleflight 会把 A 的结果发给 B
var sf singleflight.Group

// Cache 是最小缓存契约。值一律按字符串存 —— 编解码由调用方和适配器负责,
// 包本身不认识任何业务类型
type Cache interface {
	// Get 未命中必须返回 ErrMiss
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// Load 取 key 对应的值:
//
//  1. 先查缓存,命中直接返回 —— 这一步**在 singleflight 之外**,因为命中是常态,
//     不该为了去重把正常的读也串行化
//  2. 未命中则回源;同一 key 的并发调用只会真正回源一次,其余共享结果
//  3. 回源成功且 cache 非 nil 时回写缓存
//
// cache 传 nil 表示跳过缓存,只要并发去重。
//
// 回源失败的结果也会在并发调用之间共享 —— 否则失败时反而会挨个重试,
// 把本来就抖的后端再压一轮。但失败**不写缓存**,避免把一次抖动固化下来。
func Load(
	ctx context.Context,
	cache Cache,
	key string,
	ttl time.Duration,
	load func(context.Context) (string, error),
) (string, error) {
	if cache != nil {
		v, err := cache.Get(ctx, key)
		if err == nil {
			return v, nil
		}
		if !errors.Is(err, ErrMiss) {
			// 缓存故障不当成未命中:隐瞒它会让调用方以为走了缓存,降级策略该由调用方定
			return "", fmt.Errorf("读取缓存失败: %w", err)
		}
	}

	v, err, _ := sf.Do(key, func() (any, error) {
		// 剥掉发起者的 ctx:leader 的客户端断开不该让所有跟随者一起拿到取消错误。
		// 代价是丢了调用方的 deadline,所以上面自己兜了 loadTimeout
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), loadTimeout)
		defer cancel()

		val, err := load(loadCtx)
		if err != nil {
			return nil, err
		}
		if cache != nil {
			// 回写失败不影响本次返回:缓存只是加速,不是真相来源
			_ = cache.Set(loadCtx, key, val, ttl)
		}
		return val, nil
	})
	if err != nil {
		return "", err
	}
	// singleflight 的 Do 返回 any,这层断言躲不掉。但 Do 只在本函数里被调用,
	// 塞进去的一定是 load 返回的 string,所以不会失败
	return v.(string), nil
}
