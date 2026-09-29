package feed

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"feed-system/internal/model/video"
	"feed-system/internal/pkg/errs"
	"feed-system/internal/pkg/sfcache"
)

// FeedService 最新视频流业务层。
type FeedService struct {
	repo  FeedRepository
	rdb   *redis.Client
	likes LikeStatusProvider
}

// LikeStatusProvider 提供用户对一批视频的点赞状态。
type LikeStatusProvider interface {
	GetUserLikeStatuses(ctx context.Context, userID int64, videoIDs []int64) (map[int64]bool, error)
}

func NewFeedService(repo FeedRepository, rdb *redis.Client, likes LikeStatusProvider) *FeedService {
	return &FeedService{repo: repo, rdb: rdb, likes: likes}
}

const (
	latestVideosKey    = "feed:video:latest"
	latestVideosTTL    = time.Hour
	latestBackfillSize = 500
	defaultLatestLimit = 100
)

func toFeedItem(v *video.Video) *FeedItem {
	if v == nil {
		return nil
	}
	return &FeedItem{
		ID:           v.ID,
		AuthorID:     v.AuthorID,
		Username:     v.Username,
		AvatarURL:    v.AvatarURL,
		Title:        v.Title,
		Description:  v.Description,
		PlayURL:      v.PlayURL,
		CoverURL:     v.CoverURL,
		CreatedAt:    v.CreatedAt,
		Status:       v.Status,
		PlayCount:    v.PlayCount,
		LikesCount:   v.LikesCount,
		CommentCount: v.CommentCount,
	}
}

func (s *FeedService) ListLatest(ctx context.Context, req ListLatestReq, userID int64) (*ListLatestResp, error) {
	if s.rdb == nil {
		return nil, errs.ErrInternalCache
	}

	limit := req.Limit
	if limit == 0 {
		limit = defaultLatestLimit
	}

	// 取缓存里最旧的一条,只为判断 ZSET 空不空
	oldest, err := s.rdb.ZRangeWithScores(ctx, latestVideosKey, 0, 0).Result()
	if err != nil {
		return nil, errs.ErrInternal
	}

	var result []*FeedItem
	if len(oldest) == 0 {
		// sfcache 这里只用于并发去重,缓存本身由下面的 ZSET 保存。
		videos, err := sfcache.Load(ctx, nil, latestVideosKey, time.Minute,
			func(loadCtx context.Context) ([]*video.Video, error) {
				videos, err := s.repo.ListLatestVideos(loadCtx, time.Now(), latestBackfillSize)
				if err != nil {
					return nil, errs.ErrInternal
				}
				return videos, nil
			})
		if err != nil {
			return nil, errs.ErrInternal
		}

		members := make([]redis.Z, 0, len(videos))
		for _, item := range videos {
			card := toFeedItem(item)
			result = append(result, card)

			member, err := json.Marshal(card)
			if err != nil {
				slog.WarnContext(ctx, "最新流成员序列化失败,跳过", "video_id", item.ID, "err", err)
				continue
			}
			// score 只用于排序;分页游标取 member 中的 created_at。
			members = append(members, redis.Z{
				Score:  float64(item.CreatedAt.Unix()),
				Member: member,
			})
		}

		// 空 ZSET 是合法状态,没有已发布视频时不调用 ZAdd。
		if len(members) > 0 {
			// 一起设置 TTL,避免 ZADD 成功后进程退出留下永久缓存。
			pipe := s.rdb.TxPipeline()
			pipe.ZAdd(ctx, latestVideosKey, members...)
			pipe.Expire(ctx, latestVideosKey, latestVideosTTL)
			if _, err := pipe.Exec(ctx); err != nil {
				return nil, errs.ErrInternal
			}
		}
	} else {
		zSlice, err := s.rdb.ZRevRangeWithScores(ctx, latestVideosKey, 0, int64(latestBackfillSize)-1).Result()
		if err != nil {
			return nil, errs.ErrInternal
		}
		for _, z := range zSlice {
			var item FeedItem
			if err := json.Unmarshal([]byte(z.Member.(string)), &item); err != nil {
				slog.WarnContext(ctx, "最新流缓存里有解不开的成员", "err", err)
				continue
			}
			result = append(result, &item)
		}
	}

	// 首页(0)使用当前秒级时间戳。
	cursorSeconds := req.Cursor
	if cursorSeconds == 0 {
		cursorSeconds = time.Now().Unix()
	}
	cursor := time.Unix(cursorSeconds, 0)

	resp := ListLatestResp{Items: []FeedItem{}}
	var oldestCachedAt time.Time
	for _, item := range result {
		if item != nil && (oldestCachedAt.IsZero() || item.CreatedAt.Before(oldestCachedAt)) {
			oldestCachedAt = item.CreatedAt
		}
	}

	// 游标早于缓存覆盖范围时直接查数据库。
	if len(result) == 0 || !cursor.After(oldestCachedAt) {
		videos, err := s.repo.ListLatestVideos(ctx, cursor, limit)
		if err != nil {
			slog.ErrorContext(ctx, "查询最新流失败", "cursor", cursor, "err", err)
			return nil, errs.ErrInternal.WithMsg("查询最新流失败")
		}
		for _, item := range videos {
			resp.Items = append(resp.Items, *toFeedItem(item))
		}
	} else {
		for _, item := range result {
			if len(resp.Items) >= limit {
				break
			}
			if item.CreatedAt.Before(cursor) {
				resp.Items = append(resp.Items, *item)
			}
		}

		// 热数据不足一页时,从缓存边界之前查冷数据补足。
		if len(resp.Items) < limit {
			remaining := limit - len(resp.Items)
			videos, err := s.repo.ListLatestVideos(ctx, oldestCachedAt, remaining)
			if err != nil {
				return nil, errs.ErrInternal.WithMsg("查询最新流失败")
			}
			for _, item := range videos {
				resp.Items = append(resp.Items, *toFeedItem(item))
			}
		}
	}

	// 点赞状态取决于当前用户,在读完共享缓存后单独补齐,不写回 Redis。
	if userID > 0 && len(resp.Items) > 0 {
		if s.likes == nil {
			slog.ErrorContext(ctx, "视频流点赞状态查询器未配置", "user_id", userID)
			return nil, errs.ErrInternal
		}
		videoIDs := make([]int64, 0, len(resp.Items))
		for _, item := range resp.Items {
			videoIDs = append(videoIDs, item.ID)
		}
		statuses, err := s.likes.GetUserLikeStatuses(ctx, userID, videoIDs)
		if err != nil {
			// 点赞查询由提供方记录底层错误,这里仅向上返回通用内部错误,不暴露实现细节。
			return nil, errs.ErrInternal
		}
		for i := range resp.Items {
			liked := statuses[resp.Items[i].ID]
			resp.Items[i].IsLike = &liked
		}
	}

	if n := len(resp.Items); n > 0 {
		resp.NextCursor = resp.Items[n-1].CreatedAt.Unix()
	}
	return &resp, nil
}
