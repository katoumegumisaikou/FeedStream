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
