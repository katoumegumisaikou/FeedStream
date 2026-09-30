package feed

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"feed-system/internal/model/video"
)

// pagedFeedRepo 遵守 before 和 limit 的内存仓储。
//
// handler_test 里那个 latestFeedRepo 把这两个参数都忽略了,所以只能验「返回了什么」,
// 验不了分页 —— hasMore 这件事必须让仓储真的按 limit 截断才测得出来
type pagedFeedRepo struct {
	videos []*video.Video
}

func (r *pagedFeedRepo) ListLatestVideos(_ context.Context, before time.Time, limit int) ([]*video.Video, error) {
	var out []*video.Video
	for _, v := range r.videos {
		if v.CreatedAt.Before(before) {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit >= 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *pagedFeedRepo) ListVideosByLikes(context.Context, int64, int64, int) ([]*video.Video, error) {
	return nil, nil
}

// newLatestService userID 传 0 走匿名分支,setLikeStatuses 会直接返回,
// 所以第三个参数(likes 提供方)可以传 nil
func newLatestService(t *testing.T, videos []*video.Video) *FeedService {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewFeedService(&pagedFeedRepo{videos: videos}, rdb, nil)
}

func publishedAt(id int64, at time.Time) *video.Video {
	return &video.Video{ID: id, Title: "v", CreatedAt: at, Status: video.StatusPublished}
}

// TestListLatest_NextCursor 锁住「next_cursor 只在确实还有下一页时才给」。
//
// 改之前是「这页只要有数据就给」,于是最后一页也会带一个非 0 的游标,
// 前端得再请求一次空页才知道到底了。现在和 ListLike 一样多要一条来判 hasMore
func TestListLatest_NextCursor(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("还有更多时给游标", func(t *testing.T) {
		svc := newLatestService(t, []*video.Video{
			publishedAt(3, now.Add(-time.Minute)),
			publishedAt(2, now.Add(-2*time.Minute)),
			publishedAt(1, now.Add(-3*time.Minute)),
		})

		page1, err := svc.ListLatest(ctx, ListLatestReq{Limit: 2}, 0)
		require.NoError(t, err)
		require.Len(t, page1.Items, 2, "limit=2 就只给 2 条,多要的那条不进响应")
		assert.Equal(t, int64(3), page1.Items[0].ID)
		assert.Equal(t, int64(2), page1.Items[1].ID)
		require.NotZero(t, page1.NextCursor, "后面还剩一条,该给游标")

		page2, err := svc.ListLatest(ctx, ListLatestReq{Limit: 2, Cursor: page1.NextCursor}, 0)
		require.NoError(t, err)
		require.Len(t, page2.Items, 1)
		assert.Equal(t, int64(1), page2.Items[0].ID)
		assert.Equal(t, int64(0), page2.NextCursor, "取完了,游标该是 0")
	})

	t.Run("正好取满时不留游标", func(t *testing.T) {
		svc := newLatestService(t, []*video.Video{
			publishedAt(2, now.Add(-time.Minute)),
			publishedAt(1, now.Add(-2*time.Minute)),
		})

		resp, err := svc.ListLatest(ctx, ListLatestReq{Limit: 2}, 0)
		require.NoError(t, err)
		require.Len(t, resp.Items, 2)
		assert.Equal(t, int64(0), resp.NextCursor,
			"正好取满且后面没有了 —— 不该给个假游标让前端白跑一趟")
	})

	t.Run("没有视频时游标为 0", func(t *testing.T) {
		resp, err := newLatestService(t, nil).ListLatest(ctx, ListLatestReq{Limit: 2}, 0)
		require.NoError(t, err)
		assert.NotNil(t, resp.Items)
		assert.Empty(t, resp.Items)
		assert.Equal(t, int64(0), resp.NextCursor)
	})
}
