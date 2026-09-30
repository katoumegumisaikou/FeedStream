package feed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"feed-system/internal/model/video"
	"feed-system/internal/pkg/errs"
)

type latestFeedRepo struct {
	items []*video.Video
}

func (r latestFeedRepo) ListLatestVideos(context.Context, time.Time, int64, int) ([]*video.Video, error) {
	return r.items, nil
}

func (r latestFeedRepo) ListVideosByLikes(context.Context, int64, int64, int) ([]*video.Video, error) {
	return r.items, nil
}

func TestRegisterRouter_ListLatest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	miniRedis := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	t.Cleanup(func() {
		if err := rdb.Close(); err != nil {
			t.Errorf("关闭测试 Redis 客户端失败: %v", err)
		}
	})

	repo := latestFeedRepo{items: []*video.Video{{
		ID:        1,
		CreatedAt: time.Now().Add(-time.Minute),
		Status:    video.StatusPublished,
	}}}
	svc := NewFeedService(repo, rdb, nil)
	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterRouter(v1, NewFeedHandler(svc), nil, rdb)
	// 同时注册 video 路由,确认原有 /videos/latest 不会被 /videos/:id 抢走。
	video.RegisterRouter(v1, video.NewVideoHandler(video.NewVideoService(nil, nil, nil)), nil, rdb)

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{name: "公开路由返回最新视频", query: "?limit=1", wantCode: 0},
		{name: "limit 超出范围返回参数错误", query: "?limit=101", wantCode: errs.ErrInvalidParam.Code},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/videos/latest"+tt.query, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusOK, resp.Code)
			var body struct {
				Code int             `json:"code"`
				Data *ListLatestResp `json:"data"`
			}
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			assert.Equal(t, tt.wantCode, body.Code)
			if tt.wantCode == 0 {
				require.NotNil(t, body.Data)
				require.Len(t, body.Data.Items, 1)
				assert.Equal(t, int64(1), body.Data.Items[0].ID)
			}
		})
	}
}
