package video

import (
	"context"

	"gorm.io/gorm"
)

// videoRepository 视频仓储(基于 GORM)
type videoRepository struct {
	db *gorm.DB
}

// CreateVideo 插入一条视频记录(GORM 会把自增主键回填进 v.ID)
func (r *videoRepository) CreateVideo(ctx context.Context, v *Video) error {
	return r.db.WithContext(ctx).Create(v).Error
}
