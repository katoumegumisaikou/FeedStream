package sfcache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCache 内存版 Cache,记录回写次数,便于断言「有没有真的回源」
type fakeCache struct {
	mu     sync.Mutex
	data   map[string]string
	getErr error // 非 nil 时 Get 直接返回它(模拟缓存故障)
	sets   int
}

func newFakeCache() *fakeCache {
	return &fakeCache{data: map[string]string{}}
}

func (c *fakeCache) Get(ctx context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.getErr != nil {
		return "", c.getErr
	}
	v, ok := c.data[key]
	if !ok {
		return "", ErrMiss
	}
	return v, nil
}

func (c *fakeCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	c.sets++
	return nil
}

func (c *fakeCache) setCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sets
}

func TestLoad(t *testing.T) {
	ctx := context.Background()

	t.Run("缓存命中时不回源", func(t *testing.T) {
		cache := newFakeCache()
		require.NoError(t, cache.Set(ctx, "hit", "cached", time.Minute))

		var calls atomic.Int64
		v, err := Load(ctx, cache, "hit", time.Minute, func(context.Context) (string, error) {
			calls.Add(1)
			return "from-db", nil
		})

		require.NoError(t, err)
		assert.Equal(t, "cached", v)
		assert.Equal(t, int64(0), calls.Load(), "缓存命中就不该回源")
	})

	t.Run("未命中时回源并回写缓存", func(t *testing.T) {
		cache := newFakeCache()

		v, err := Load(ctx, cache, "miss", time.Minute, func(context.Context) (string, error) {
			return "from-db", nil
		})

		require.NoError(t, err)
		assert.Equal(t, "from-db", v)
		assert.Equal(t, 1, cache.setCount(), "回源结果应写回缓存")
		assert.Equal(t, "from-db", cache.data["miss"])
	})

	t.Run("cache 为 nil 时只去重不缓存", func(t *testing.T) {
		v, err := Load(ctx, nil, "nocache", time.Minute, func(context.Context) (string, error) {
			return "from-db", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "from-db", v)
	})

	t.Run("缓存故障不当成未命中", func(t *testing.T) {
		cache := newFakeCache()
		cache.getErr = errors.New("redis down")

		var calls atomic.Int64
		_, err := Load(ctx, cache, "broken", time.Minute, func(context.Context) (string, error) {
			calls.Add(1)
			return "x", nil
		})

		assert.Error(t, err)
		assert.Equal(t, int64(0), calls.Load(), "缓存故障该直接报错,而不是偷偷回源")
	})
}

// TestLoad_并发去重 是这个包存在的理由:N 个并发请求同时缓存未命中,
// 只能有一次真正回源,其余共享那一份结果。
//
// 也顺带锁住「去重域是进程级的」这条:所有调用都走同一个包级入口,
// 没有句柄可以拿,所以不存在「在调用点新建一个域、把去重关掉」这种写法
func TestLoad_并发去重(t *testing.T) {
	const n = 50
	cache := newFakeCache()

	var calls atomic.Int64
	results := make([]string, n)
	errsCh := make([]error, n)

	start := make(chan struct{}) // 卡住所有 goroutine,让它们同时冲进去
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errsCh[i] = Load(context.Background(), cache, "concurrent", time.Minute,
				func(context.Context) (string, error) {
					calls.Add(1)
					time.Sleep(50 * time.Millisecond) // 拉宽窗口,保证跟随者都在飞行中到达
					return "shared", nil
				})
		}(i)
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int64(1), calls.Load(), "同一 key 的并发加载只该回源一次")
	for i := range n {
		require.NoError(t, errsCh[i])
		assert.Equal(t, "shared", results[i], "所有请求者应拿到同一份结果")
	}
	assert.Equal(t, 1, cache.setCount(), "回写也该只发生一次")
}

func TestLoad_不同key互不影响(t *testing.T) {
	const n = 10

	var calls atomic.Int64
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = Load(context.Background(), nil, string(rune('a'+i)), time.Minute,
				func(context.Context) (string, error) {
					calls.Add(1)
					time.Sleep(30 * time.Millisecond)
					return "v", nil
				})
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int64(n), calls.Load(), "不同 key 各回源一次,不该被合并")
}

func TestLoad_失败也被共享(t *testing.T) {
	const n = 20
	cache := newFakeCache()

	var calls atomic.Int64
	start := make(chan struct{})
	errsCh := make([]error, n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errsCh[i] = Load(context.Background(), cache, "boom", time.Minute,
				func(context.Context) (string, error) {
					calls.Add(1)
					time.Sleep(50 * time.Millisecond)
					return "", errors.New("db down")
				})
		}(i)
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int64(1), calls.Load(), "失败时反而更该合并,否则会把抖动的后端再压一轮")
	for i := range n {
		assert.Error(t, errsCh[i])
	}
	assert.Equal(t, 0, cache.setCount(), "失败结果不该写进缓存")
}

// TestLoad_发起者取消不连累跟随者 锁住 context.WithoutCancel 那条:
// leader 的客户端一断开,不能让所有跟随者一起拿到取消错误
func TestLoad_发起者取消不连累跟随者(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	done := make(chan struct{})
	var loadCtxErr error

	go func() {
		defer close(done)
		_, _ = Load(ctx, nil, "cancel-me", time.Minute, func(loadCtx context.Context) (string, error) {
			close(started)
			time.Sleep(50 * time.Millisecond)
			loadCtxErr = loadCtx.Err() // 回源拿到的 ctx 应该还是活的
			return "v", nil
		})
	}()

	<-started
	cancel() // 模拟这个请求的客户端断开
	<-done   // done 关闭后读 loadCtxErr,时序上安全

	assert.NoError(t, loadCtxErr, "回源用的 ctx 被剥掉了取消,不该跟着发起者一起取消")
}
