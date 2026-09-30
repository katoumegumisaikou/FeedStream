package feed

import (
	"context"
	"time"

	"gorm.io/gorm"

	"feed-system/internal/model/video"
)

// FeedRepository 提供视频流所需的数据查询。
type FeedRepository interface {
	// ListLatestVideos 按 (created_at, id) 倒序取已发布视频。
	//
	// beforeID 为 0 表示首页,不设边界;否则只取严格排在 (before, beforeID) 之后的那批。
	// 用行值比较 (created_at, id) < (?, ?),和 ORDER BY 逐列对应,
	// 正好命中 008 迁移的 idx_videos_create_time_id。
	//
	// 只带 before 的话,同一时刻发布的视频没有确定顺序,翻页会漏条或重复
	ListLatestVideos(ctx context.Context, before time.Time, beforeID int64, limit int) ([]*video.Video, error)
	// ListVideosByLikes 按点赞数降序分页查询已发布视频。
	ListVideosByLikes(ctx context.Context, cursorLikesCount, cursorVideoID int64, limit int) ([]*video.Video, error)
}

type feedRepository struct {
	db *gorm.DB
}

var _ FeedRepository = (*feedRepository)(nil)

func NewFeedRepository(db *gorm.DB) FeedRepository {
	return &feedRepository{db: db}
}

// ListLatestVideos 不加 deleted_at 条件:GORM 认出 video.Video 里嵌的 gorm.DeletedAt
// 会自动补上软删除过滤
func (r *feedRepository) ListLatestVideos(ctx context.Context, before time.Time, beforeID int64, limit int) ([]*video.Video, error) {
	query := r.db.WithContext(ctx).
		Where("status = ?", video.StatusPublished)
	// 行值比较:同一时刻发布的视频靠 id 决出先后,边界才没有缝
	if beforeID > 0 {
		query = query.Where("(created_at, id) < (?, ?)", before, beforeID)
	}

	var videos []*video.Video
	err := query.
		Order("created_at DESC").
		Order("id DESC").
		Limit(limit).
		Find(&videos).Error
	if err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *feedRepository) ListVideosByLikes(ctx context.Context, cursorLikesCount, cursorVideoID int64, limit int) ([]*video.Video, error) {
	query := r.db.WithContext(ctx).Model(&video.Video{}).
		Where("status = ?", video.StatusPublished)
	if cursorVideoID > 0 {
		query = query.Where(
			"(likes_count < ? OR (likes_count = ? AND id < ?))",
			cursorLikesCount, cursorLikesCount, cursorVideoID,
		)
	}

	var videos []*video.Video
	err := query.
		Order("likes_count DESC").
		Order("id DESC").
		Limit(limit).
		Find(&videos).Error
	if err != nil {
		return nil, err
	}
	return videos, nil
}
