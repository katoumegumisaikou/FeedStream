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

// FeedService Feed 流业务层。
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

func (s *FeedService) setLikeStatuses(ctx context.Context, userID int64, items []FeedItem) error {
	if userID <= 0 || len(items) == 0 {
		return nil
	}
	if s.likes == nil {
		slog.ErrorContext(ctx, "视频流点赞状态查询器未配置", "user_id", userID)
		return errs.ErrInternal
	}

	videoIDs := make([]int64, 0, len(items))
	for _, item := range items {
		videoIDs = append(videoIDs, item.ID)
	}
	statuses, err := s.likes.GetUserLikeStatuses(ctx, userID, videoIDs)
	if err != nil {
		// 点赞查询由提供方记录底层错误,这里仅返回通用内部错误。
		return errs.ErrInternal
	}
	for i := range items {
		liked := statuses[items[i].ID]
		items[i].IsLike = &liked
	}
	return nil
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

	// 点赞状态只加在响应副本上,不写入共享 Redis 缓存。
	if err := s.setLikeStatuses(ctx, userID, resp.Items); err != nil {
		return nil, err
	}

	if n := len(resp.Items); n > 0 {
		resp.NextCursor = resp.Items[n-1].CreatedAt.Unix()
	}
	return &resp, nil
}

func (s *FeedService) ListLike(ctx context.Context, req ListLikeReq, userID int64) (*ListLikeResp, error) {
	limit := req.Limit
	if limit == 0 {
		limit = defaultLatestLimit
	}
	if limit < 1 || limit > defaultLatestLimit {
		return nil, errs.ErrInvalidParam.WithMsg("每页数量无效")
	}

	var cursorLikesCount, cursorVideoID int64
	switch {
	case req.CursorLikesCount == nil && req.CursorVideoID == nil:
		// 首页不带游标。
	case req.CursorLikesCount == nil || req.CursorVideoID == nil:
		return nil, errs.ErrInvalidParam.WithMsg("分页游标无效")
	default:
		cursorLikesCount = *req.CursorLikesCount
		cursorVideoID = *req.CursorVideoID
		if cursorLikesCount < 0 || cursorVideoID <= 0 {
			return nil, errs.ErrInvalidParam.WithMsg("分页游标无效")
		}
	}

	videos, err := s.repo.ListVideosByLikes(ctx, cursorLikesCount, cursorVideoID, limit+1)
	if err != nil {
		slog.ErrorContext(ctx, "按点赞数查询视频流失败", "cursor_likes_count", cursorLikesCount, "cursor_video_id", cursorVideoID, "err", err)
		return nil, errs.ErrInternal
	}

	hasMore := len(videos) > limit
	if hasMore {
		videos = videos[:limit]
	}

	resp := ListLikeResp{Items: make([]FeedItem, 0, len(videos))}
	for _, item := range videos {
		if card := toFeedItem(item); card != nil {
			resp.Items = append(resp.Items, *card)
		}
	}
	if hasMore && len(resp.Items) > 0 {
		last := resp.Items[len(resp.Items)-1]
		resp.NextCursor = &LikeCursor{LikesCount: last.LikesCount, VideoID: last.ID}
	}
	if err := s.setLikeStatuses(ctx, userID, resp.Items); err != nil {
		return nil, err
	}
	return &resp, nil
}
