package feed

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
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
	// 多要一条:能取到就说明后面还有,这时才给 cursor。
	// 和 ListLike 同一套判据 —— next_cursor 的「有没有」立刻是准的,
	// 前端不用为了确认到底而多请求一次空页
	want := limit + 1

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
				// beforeID 传 0:回填要的是「最新一批」,不设上界
				videos, err := s.repo.ListLatestVideos(loadCtx, time.Now(), 0, latestBackfillSize)
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

	// ZSET 的 score 只到秒,同一秒的成员 Redis 按 member 字典序排 —— 那个顺序和
	// (created_at, id) 无关。缓存里成员不多(≤ latestBackfillSize),统一再排一次,
	// 让内存里的顺序和 SQL 的 ORDER BY created_at DESC, id DESC 完全一致;
	// 否则同一时刻的几条会在翻页时重复出现
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		if am, bm := a.CreatedAt.UnixMicro(), b.CreatedAt.UnixMicro(); am != bm {
			return am > bm
		}
		return a.ID > b.ID
	})

	// 游标是 (created_at, id) 复合的:两个字段要么都传要么都不传,
	// 只传一个的话边界不完整,那还不如当首页处理
	var (
		hasCursor bool
		cursorAt  time.Time
		cursorID  int64
	)
	switch {
	case req.CursorCreatedAt == nil && req.CursorVideoID == nil:
		// 首页,不设边界
	case req.CursorCreatedAt == nil || req.CursorVideoID == nil:
		return nil, errs.ErrInvalidParam.WithMsg("分页游标无效")
	default:
		if *req.CursorVideoID <= 0 {
			return nil, errs.ErrInvalidParam.WithMsg("分页游标无效")
		}
		hasCursor = true
		cursorAt = time.UnixMicro(*req.CursorCreatedAt)
		cursorID = *req.CursorVideoID
	}

	resp := ListLatestResp{Items: []FeedItem{}}

	// 缓存里最旧的那条(按 created_at DESC, id DESC 排最后),用来判断缓存覆盖到哪
	var (
		oldestAt time.Time
		oldestID int64
	)
	for _, item := range result {
		if item == nil {
			continue
		}
		if oldestID == 0 || older(item.CreatedAt, item.ID, oldestAt, oldestID) {
			oldestAt, oldestID = item.CreatedAt, item.ID
		}
	}

	// 游标落在缓存覆盖范围之外(比缓存最旧的还旧)时,直接查数据库
	if len(result) == 0 || (hasCursor && older(cursorAt, cursorID, oldestAt, oldestID)) {
		videos, err := s.repo.ListLatestVideos(ctx, cursorAt, cursorID, want)
		if err != nil {
			slog.ErrorContext(ctx, "查询最新流失败", "cursor_at", cursorAt, "cursor_id", cursorID, "err", err)
			return nil, errs.ErrInternal.WithMsg("查询最新流失败")
		}
		for _, item := range videos {
			resp.Items = append(resp.Items, *toFeedItem(item))
		}
	} else {
		for _, item := range result {
			if len(resp.Items) >= want {
				break
			}
			if !hasCursor || older(item.CreatedAt, item.ID, cursorAt, cursorID) {
				resp.Items = append(resp.Items, *item)
			}
		}

		// 热数据不足一页时,从缓存边界之前查冷数据补足
		if len(resp.Items) < want {
			remaining := want - len(resp.Items)
			videos, err := s.repo.ListLatestVideos(ctx, oldestAt, oldestID, remaining)
			if err != nil {
				return nil, errs.ErrInternal.WithMsg("查询最新流失败")
			}
			for _, item := range videos {
				resp.Items = append(resp.Items, *toFeedItem(item))
			}
		}
	}

	// 多要的那条只用来判「还有没有更多」,不进响应
	hasMore := len(resp.Items) > limit
	if hasMore {
		resp.Items = resp.Items[:limit]
	}

	// 点赞状态只加在响应副本上,不写入共享 Redis 缓存。
	if err := s.setLikeStatuses(ctx, userID, resp.Items); err != nil {
		return nil, err
	}

	// 只在确实还有下一页时给游标;到底了就是 nil(见 ListLatestResp 注释)
	if hasMore && len(resp.Items) > 0 {
		last := resp.Items[len(resp.Items)-1]
		resp.NextCursor = &FeedCursor{CreatedAt: last.CreatedAt.UnixMicro(), VideoID: last.ID}
	}
	return &resp, nil
}

// older 判断 (at, id) 是否严格排在 (thanAt, thanID) 之后,也就是更旧的那一侧。
// 与 SQL 的 (created_at, id) < (?, ?) 同一语义:先比时间,时间相同再比 id。
//
// 时间只比到微秒 —— created_at 列是 TIMESTAMP(微秒),游标也按微秒传。
// 直接比 time.Time 的话,值里残留的纳秒会让本该相等的一对判成不等,
// 那些视频会被当成「不是更旧」而静默跳过,翻页整批漏掉
func older(at time.Time, id int64, thanAt time.Time, thanID int64) bool {
	am, bm := at.UnixMicro(), thanAt.UnixMicro()
	if am != bm {
		return am < bm
	}
	return id < thanID
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
