package video

import (
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

	"feed-system/internal/pkg/errs"
)

func newLatestTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	miniRedis := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	t.Cleanup(func() {
		if err := rdb.Close(); err != nil {
			t.Errorf("关闭测试 Redis 客户端失败: %v", err)
		}
	})

	video := publishedVideo()
	video.CreatedAt = time.Now().Add(-time.Minute)
	repo := &fakeVideoRepo{videos: map[int64]*Video{video.ID: video}}
	svc := NewVideoService(repo, rdb, nil)
	router := gin.New()
	RegisterRouter(router.Group("/api/v1"), NewVideoHandler(svc), nil, rdb)
	return router
}

func TestRegisterRouter_ListLatest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		wantCode   int
		wantStatus int
	}{
		{
			name:       "公开路由返回最新视频",
			query:      "?limit=1",
			wantCode:   0,
			wantStatus: http.StatusOK,
		},
		{
			name:       "limit 超出范围返回参数错误",
			query:      "?limit=101",
			wantCode:   errs.ErrInvalidParam.Code,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newLatestTestRouter(t)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/videos/latest"+tt.query, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.wantStatus, resp.Code)

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
