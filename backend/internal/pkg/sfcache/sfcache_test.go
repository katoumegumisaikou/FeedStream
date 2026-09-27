package sfcache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRedis 起一个内存 Redis 并返回客户端与它的直接句柄。
//
// 本包现在直连 *redis.Client,测试也走真实协议 —— 好处是「命令有没有拼对」这类
// 问题才测得出来。mr 用来绕过协议预置/读取值,不用写一遍 GET。
//
// 代价:回写「次数」数不出来(miniredis 没有命令计数),只能断言值对不对。
// 需要断言次数的用例都改成断言「回源次数」,那才是要守的东西
func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func TestLoad(t *testing.T) {
	ctx := context.Background()

	t.Run("缓存命中时不回源", func(t *testing.T) {
		rdb, mr := newTestRedis(t)
		// 预置的值必须是 JSON —— sfcache 存进去的就是 JSON,手工塞值也得守这个格式
		require.NoError(t, mr.Set("hit", `"cached"`))

		var calls atomic.Int64
		v, err := Load(ctx, rdb, "hit", time.Minute, func(context.Context) (string, error) {
			calls.Add(1)
			return "from-db", nil
		})

		require.NoError(t, err)
		assert.Equal(t, "cached", v)
		assert.Equal(t, int64(0), calls.Load(), "缓存命中就不该回源")
	})

	t.Run("未命中时回源并回写缓存", func(t *testing.T) {
		rdb, mr := newTestRedis(t)

		v, err := Load(ctx, rdb, "miss", time.Minute, func(context.Context) (string, error) {
			return "from-db", nil
		})

		require.NoError(t, err)
		assert.Equal(t, "from-db", v)
		// 缓存里放的是 JSON,不是裸值 —— 编解码是包的事,调用方不该看到
		stored, err := mr.Get("miss")
		require.NoError(t, err, "回源结果应写回缓存")
		assert.Equal(t, `"from-db"`, stored)
	})

	t.Run("rdb 为 nil 时只去重不缓存", func(t *testing.T) {
		v, err := Load(ctx, nil, "nocache", time.Minute, func(context.Context) (string, error) {
			return "from-db", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "from-db", v)
	})

	// 这条锁的是降级策略:Redis 挂了不该把鉴权、文件路由一起带下去。
	// 宁可每次都回源(慢),也不要整站 401/404
	t.Run("缓存故障降级回源,不把错误抛给调用方", func(t *testing.T) {
		rdb, mr := newTestRedis(t)
		mr.SetError("MASTERDOWN Link with MASTER is down")

		var calls atomic.Int64
		v, err := Load(ctx, rdb, "broken", time.Minute, func(context.Context) (string, error) {
			calls.Add(1)
			return "from-db", nil
		})

		require.NoError(t, err, "缓存故障不该让调用方拿到错误")
		assert.Equal(t, "from-db", v)
		assert.Equal(t, int64(1), calls.Load(), "缓存故障该回源")
	})
}

// TestLoad_并发去重 是这个包存在的理由:N 个并发请求同时缓存未命中,
// 只能有一次真正回源,其余共享那一份结果。
//
// 也顺带锁住「去重域是进程级的」这条:所有调用都走同一个包级入口,
// 没有句柄可以拿,所以不存在「在调用点新建一个域、把去重关掉」这种写法
func TestLoad_并发去重(t *testing.T) {
	const n = 50
	rdb, mr := newTestRedis(t)

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
			results[i], errsCh[i] = Load(context.Background(), rdb, "concurrent", time.Minute,
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
	// 回写次数数不出来(miniredis 不计数),这里只验写进去了且值正确。
	// 「只回源一次」才是这条用例真正守的东西,那个由上面的 calls 断言
	stored, err := mr.Get("concurrent")
	require.NoError(t, err)
	assert.Equal(t, `"shared"`, stored)
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
	rdb, mr := newTestRedis(t)

	var calls atomic.Int64
	start := make(chan struct{})
	errsCh := make([]error, n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errsCh[i] = Load(context.Background(), rdb, "boom", time.Minute,
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
	assert.False(t, mr.Exists("boom"), "失败结果不该写进缓存")
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

// cacheTestTarget 模仿真实调用方缓存的业务结构体
type cacheTestTarget struct {
	AuthorID int64 `json:"author_id"`
	Status   int8  `json:"status"`
	Found    bool  `json:"found"`
}

// TestLoad_结构体原样往返 泛型的意义:调用方拿到的是自己的类型,不用手写解析
func TestLoad_结构体原样往返(t *testing.T) {
	ctx := context.Background()
	rdb, mr := newTestRedis(t)

	want := cacheTestTarget{AuthorID: 7, Status: 1, Found: true}
	got, err := Load(ctx, rdb, "struct", time.Minute, func(context.Context) (cacheTestTarget, error) {
		return want, nil
	})
	require.NoError(t, err)
	assert.Equal(t, want, got)

	stored, err := mr.Get("struct")
	require.NoError(t, err)
	assert.JSONEq(t, `{"author_id":7,"status":1,"found":true}`, stored, "缓存里该是 JSON")

	// 第二次:命中缓存,拿回来的仍是结构体本身,不是 map
	again, err := Load(ctx, rdb, "struct", time.Minute, func(context.Context) (cacheTestTarget, error) {
		t.Error("第二次该命中缓存,不该回源")
		return cacheTestTarget{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, want, again)
	assert.Equal(t, int8(1), again.Status, "int8 经 JSON 往返不该丢")
}

// TestLoad_未导出字段会静默编成空对象 把 Load 文档里那条警告变成可执行的约束。
//
// 这是本包唯一一个「不报错但数据是错的」的坑:JSON 看不见未导出字段,
// 全未导出字段的结构体会被编成 {},命中缓存时解出零值,调用方毫无察觉。
// 断言的是当前的坏行为 —— 哪天换成能处理未导出字段的编解码,这条会失败,
// 那正是应该被告知的时刻
func TestLoad_未导出字段会静默编成空对象(t *testing.T) {
	type badTarget struct {
		authorID int64 // 小写 = 未导出,编不出来
		found    bool
	}
	ctx := context.Background()
	rdb, mr := newTestRedis(t)

	_, err := Load(ctx, rdb, "bad", time.Minute, func(context.Context) (badTarget, error) {
		return badTarget{authorID: 7, found: true}, nil
	})
	require.NoError(t, err)

	stored, err := mr.Get("bad")
	require.NoError(t, err)
	assert.Equal(t, "{}", stored, "未导出字段编不出来,而且不会报错")

	got, err := Load(ctx, rdb, "bad", time.Minute, func(context.Context) (badTarget, error) {
		t.Error("不该回源")
		return badTarget{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, badTarget{}, got, "全零值,且全程没有任何报错")
}

// TestLoad_同名key被不同类型复用 同一个 key 上换了类型是编码错误。
// singleflight 会把 leader 的结果发给跟随者,直接断言会 panic ——
// 这里锁住它被降级成带 key 的 error
func TestLoad_同名key被不同类型复用(t *testing.T) {
	var loads atomic.Int64

	started := make(chan struct{})
	var wg sync.WaitGroup

	// leader:回源慢一点,好让跟随者挤进同一轮
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = Load(context.Background(), nil, "dup-key", time.Minute,
			func(context.Context) (cacheTestTarget, error) {
				close(started)
				time.Sleep(80 * time.Millisecond)
				loads.Add(1)
				return cacheTestTarget{AuthorID: 7, Found: true}, nil
			})
	}()
	<-started

	// 跟随者:同一个 key,不同的 T
	var followerErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, followerErr = Load(context.Background(), nil, "dup-key", time.Minute,
			func(context.Context) (int64, error) {
				loads.Add(1)
				return 42, nil
			})
	}()
	wg.Wait()

	require.Error(t, followerErr, "类型对不上该返回错误,而不是 panic")
	assert.Contains(t, followerErr.Error(), "dup-key", "错误里要带上 key,否则没法定位")
	assert.Equal(t, int64(1), loads.Load(), "跟随者该共享 leader 的结果,不回源")
}
