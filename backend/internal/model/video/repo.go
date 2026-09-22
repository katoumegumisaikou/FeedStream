package video

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// VideoRepository 视频仓储接口(对外暴露的唯一契约)。
// 声明成接口而不是结构体:service 依赖抽象,测试里才能注入内存实现
type VideoRepository interface {
	// CreateVideo 插入一条记录,GORM 会把自增主键回填进 v.ID
	CreateVideo(ctx context.Context, v *Video) error

	// FindVideoByID 按主键查,查不到返回 ErrNotFound
	FindVideoByID(ctx context.Context, id int64) (*Video, error)

	// FindVideoByPlayURL 按播放地址反查,查不到返回 ErrNotFound。
	// 公开文件路由只有 URL 没有 ID,靠它拿到视频状态才能判可见性(走 006 迁移的
	// idx_videos_play_url,是索引命中不是全表扫)
	FindVideoByPlayURL(ctx context.Context, playURL string) (*Video, error)

	// SavePlayReport 在一个事务里追加一条播放流水,并把 videos.play_count 加一。
	// 两件事必须同生共死:流水是明细、play_count 是读列表直接用的聚合,
	// 只成一件就会出现「有明细没计数」这种对不上的漂移
	SavePlayReport(ctx context.Context, r *PlayRecord) error

	// UpdateVideoFields 只更新传入的列,updated_at 由 GORM 自动填。
	// 用 map 而不是 struct:struct 更新会跳过零值,把 Description 清空成 "" 就写不进库
	UpdateVideoFields(ctx context.Context, id int64, fields map[string]any) error
}

// ErrNotFound 仓储层「没查到」的哨兵错误。
// 不把 gorm.ErrRecordNotFound 透出去:它会一路冒到 response.Error,
// 而那里的类型断言认不出它不是 ServiceErr,最后只会给前端「未知错误」
var ErrNotFound = errors.New("video: not found")

// videoRepository 基于 GORM 的实现(包内私有,外部只能通过接口访问)
type videoRepository struct {
	db *gorm.DB
}

// 编译期断言:确保 videoRepository 实现 VideoRepository 接口
var _ VideoRepository = (*videoRepository)(nil)

// NewVideoRepository 构造 VideoRepository;返回接口类型以隐藏 GORM 实现细节
func NewVideoRepository(db *gorm.DB) VideoRepository {
	return &videoRepository{db: db}
}

func (r *videoRepository) CreateVideo(ctx context.Context, v *Video) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *videoRepository) FindVideoByID(ctx context.Context, id int64) (*Video, error) {
	var v Video
	err := r.db.WithContext(ctx).First(&v, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *videoRepository) FindVideoByPlayURL(ctx context.Context, playURL string) (*Video, error) {
	var v Video
	err := r.db.WithContext(ctx).Where("play_url = ?", playURL).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// SavePlayReport 流水与计数必须一起成功,所以包一个事务:
// 分开写会在中间失败时留下「有明细没计数」的漂移,而 play_count 没有自愈途径
func (r *videoRepository) SavePlayReport(ctx context.Context, rec *PlayRecord) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(rec).Error; err != nil {
			return err
		}
		return tx.Model(&Video{}).
			Where("id = ?", rec.VideoID).
			// UpdateColumn 而非 Update:播放数涨了不等于内容被改动,不该顺带顶掉 updated_at
			UpdateColumn("play_count", gorm.Expr("play_count + 1")).Error
	})
}

func (r *videoRepository) UpdateVideoFields(ctx context.Context, id int64, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", id).Updates(fields).Error
}
