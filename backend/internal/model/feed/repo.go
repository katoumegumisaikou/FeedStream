package feed

import (
	"context"
	"time"

	"gorm.io/gorm"

	"feed-system/internal/model/video"
)

// FeedRepository 提供视频流所需的数据查询。
type FeedRepository interface {
	ListLatestVideos(ctx context.Context, before time.Time, limit int) ([]*video.Video, error)
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

// ListLatestVideos 只查询已发布、且创建时间早于 before 的视频,按时间倒序返回。
func (r *feedRepository) ListLatestVideos(ctx context.Context, before time.Time, limit int) ([]*video.Video, error) {
	var videos []*video.Video
	err := r.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", video.StatusPublished, before).
		Order("created_at DESC").
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
