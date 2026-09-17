package video

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"feed-system/internal/pkg/errs"
)

func TestValidCoverURL(t *testing.T) {
	allow := []string{
		"/covers/abc.png",
		"/covers/2026/09/x.jpg",
	}
	// 这几种一旦放进去,前端拿它当 img src 就会出事
	reject := []string{
		"",                          // 空串由调用方跳过,不归这个函数管
		"covers/abc.png",            // 缺前导 /
		"/covers",                   // 只有前缀,没有具体文件
		"/coversabc/x.png",          // 前缀不完整
		"/covers/../../etc/passwd",  // 目录穿越
		"http://evil.com/x.png",     // 外站
		"//evil.com/x.png",          // 协议相对,浏览器会当外站
		"javascript:alert(1)",       // 直接就是脚本
		"data:image/svg+xml,<svg/>", // data URL
	}

	for _, u := range allow {
		assert.True(t, validCoverURL(u), "应放行: %s", u)
	}
	for _, u := range reject {
		assert.False(t, validCoverURL(u), "应拦下: %s", u)
	}
}

func TestVideoIDParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		param   string
		wantID  int64
		wantErr bool
	}{
		{"7", 7, false},
		{"0", 0, true},                    // 0 不是合法主键
		{"-1", 0, true},                   // 负数
		{"abc", 0, true},                  // 非数字
		{"", 0, true},                     // 参数缺失
		{"99999999999999999999", 0, true}, // 超出 int64
	}

	for _, tc := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Params = gin.Params{{Key: "id", Value: tc.param}}

		id, err := videoIDParam(c)
		if tc.wantErr {
			assert.Error(t, err, "应报错: %q", tc.param)
			continue
		}
		assert.NoError(t, err, "不该报错: %q", tc.param)
		assert.Equal(t, tc.wantID, id)
	}
}

// fakeVideoRepo 内存版 VideoRepository。
// 仓储提成接口后,归属与状态这类业务规则不用连 PG 就能测
type fakeVideoRepo struct {
	videos     map[int64]*Video
	lastFields map[string]any // 记下最后一次写了哪些列
	updateErr  error
}

func (f *fakeVideoRepo) CreateVideo(ctx context.Context, v *Video) error { return nil }

func (f *fakeVideoRepo) FindVideoByID(ctx context.Context, id int64) (*Video, error) {
	if v, ok := f.videos[id]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}

func (f *fakeVideoRepo) UpdateVideoFields(ctx context.Context, id int64, fields map[string]any) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.lastFields = fields
	return nil
}

const testUserID int64 = 7

func draftVideo() *Video {
	return &Video{
		ID:       1,
		AuthorID: testUserID,
		Title:    "原始文件名.mp4",
		PlayURL:  "/videos/7/a.mp4",
		Status:   StatusDraft,
	}
}

// newTestService 只测编辑与发布这两条路径 —— 它们不碰 rdb / users,所以传 nil
func newTestService(v *Video) (*VideoService, *fakeVideoRepo) {
	repo := &fakeVideoRepo{videos: map[int64]*Video{}}
	if v != nil {
		repo.videos[v.ID] = v
	}
	return NewVideoService(repo, nil, nil), repo
}

func assertCode(t *testing.T, err error, want errs.ServiceErr) {
	t.Helper()
	require.Error(t, err)
	got, ok := errs.As(err)
	require.True(t, ok, "应该返回 ServiceErr,实际 %T: %v", err, err)
	assert.Equal(t, want.Code, got.Code, "错误码不符: %v", err)
}

func TestUpdateVideo(t *testing.T) {
	ctx := context.Background()

	t.Run("更新标题与简介", func(t *testing.T) {
		svc, repo := newTestService(draftVideo())

		resp, err := svc.UpdateVideo(ctx, 1, testUserID, UpdateVideoReq{Title: "新标题", Description: "简介"})
		require.NoError(t, err)
		assert.Equal(t, "新标题", resp.Title)
		assert.Equal(t, "简介", resp.Description)
		assert.Equal(t, "新标题", repo.lastFields["title"])
	})

	t.Run("cover_url 留空时不动原封面", func(t *testing.T) {
		v := draftVideo()
		v.CoverURL = "/covers/old.png"
		svc, repo := newTestService(v)

		resp, err := svc.UpdateVideo(ctx, 1, testUserID, UpdateVideoReq{Title: "t"})
		require.NoError(t, err)
		assert.Equal(t, "/covers/old.png", resp.CoverURL)
		_, wrote := repo.lastFields["cover_url"]
		assert.False(t, wrote, "留空就不该把 cover_url 放进更新列")
	})

	t.Run("封面不是站内路径被拒", func(t *testing.T) {
		svc, _ := newTestService(draftVideo())
		for _, bad := range []string{"http://evil.com/x.png", "javascript:alert(1)", "/covers/../secret"} {
			_, err := svc.UpdateVideo(ctx, 1, testUserID, UpdateVideoReq{Title: "t", CoverURL: bad})
			assertCode(t, err, errs.ErrInvalidParam)
		}
	})

	t.Run("非作者 403", func(t *testing.T) {
		svc, _ := newTestService(draftVideo())
		_, err := svc.UpdateVideo(ctx, 1, 999, UpdateVideoReq{Title: "t"})
		assertCode(t, err, errs.ErrForbidden)
	})

	t.Run("视频不存在 404", func(t *testing.T) {
		svc, _ := newTestService(nil)
		_, err := svc.UpdateVideo(ctx, 42, testUserID, UpdateVideoReq{Title: "t"})
		assertCode(t, err, errs.ErrNotFound)
	})

	t.Run("已下架不能编辑", func(t *testing.T) {
		v := draftVideo()
		v.Status = StatusRemoved
		svc, _ := newTestService(v)
		_, err := svc.UpdateVideo(ctx, 1, testUserID, UpdateVideoReq{Title: "t"})
		assertCode(t, err, errs.ErrConflict)
	})

	t.Run("仓储出错", func(t *testing.T) {
		svc, repo := newTestService(draftVideo())
		repo.updateErr = errors.New("boom")
		_, err := svc.UpdateVideo(ctx, 1, testUserID, UpdateVideoReq{Title: "t"})
		assertCode(t, err, errs.ErrInternal)
	})
}

func TestPublishVideo(t *testing.T) {
	ctx := context.Background()

	t.Run("草稿可以发布", func(t *testing.T) {
		svc, repo := newTestService(draftVideo())

		resp, err := svc.PublishVideo(ctx, 1, testUserID)
		require.NoError(t, err)
		assert.Equal(t, StatusPublished, resp.Status)
		assert.Equal(t, StatusPublished, repo.lastFields["status"])
	})

	t.Run("重复发布幂等且不再写库", func(t *testing.T) {
		v := draftVideo()
		v.Status = StatusPublished
		svc, repo := newTestService(v)

		resp, err := svc.PublishVideo(ctx, 1, testUserID)
		require.NoError(t, err)
		assert.Equal(t, StatusPublished, resp.Status)
		assert.Nil(t, repo.lastFields, "已发布的再调一次不该产生写操作")
	})

	t.Run("已下架不能发布", func(t *testing.T) {
		v := draftVideo()
		v.Status = StatusRemoved
		svc, _ := newTestService(v)
		_, err := svc.PublishVideo(ctx, 1, testUserID)
		assertCode(t, err, errs.ErrConflict)
	})

	t.Run("非作者 403", func(t *testing.T) {
		svc, _ := newTestService(draftVideo())
		_, err := svc.PublishVideo(ctx, 1, 999)
		assertCode(t, err, errs.ErrForbidden)
	})
}
