package video

import (
	"time"

	"gorm.io/gorm"
)

// 视频状态。数值不能重排:DB 列是 SMALLINT,已有数据按当前数值解释,
// 新增状态一律往后追加,不要插中间
const (
	StatusTranscoding     int8 = iota // 0 转码中(尚未接入转码,当前没有代码写入这个值)
	StatusPublished                   // 1 已发布
	StatusTranscodeFailed             // 2 转码失败(同上,预留)
	StatusRemoved                     // 3 已下架
	StatusDraft                       // 4 草稿:分片上传完成,但还没编辑标题/封面
)

type Video struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"   json:"id"`
	AuthorID    int64  `gorm:"index;not null"             json:"author_id"`
	Username    string `gorm:"type:varchar(255);not null" json:"username"`             // 冗余,列表免 JOIN
	AvatarURL   string `gorm:"type:varchar(512)"          json:"avatar_url,omitempty"` // 冗余,可空
	Title       string `gorm:"type:varchar(255);not null" json:"title"`
	Description string `gorm:"type:varchar(1000)"         json:"description,omitempty"`
	PlayURL     string `gorm:"type:varchar(255);not null" json:"play_url"`
	CoverURL    string `gorm:"type:varchar(255);not null" json:"cover_url"`

	// CreatedAt / UpdatedAt 是 GORM 约定名,按名字自动识别填充,不需要 autoCreateTime tag
	CreatedAt time.Time      `gorm:"index:idx_videos_create_time,sort:desc;index:idx_videos_popularity_time_id,priority:2,sort:desc" json:"created_at"`
	UpdatedAt time.Time      `                                                                                                       json:"updated_at"`
	DeletedAt gorm.DeletedAt `                                                                                                       json:"-"` // 软删除,不暴露前端

	Status       int8  `gorm:"not null;default:0;index"                                              json:"status"` // 状态(0 转码中 / 1 已发布 / 2 转码失败 / 3 已下架)
	PlayCount    int64 `gorm:"not null;default:0"                                                    json:"play_count"`
	LikesCount   int64 `gorm:"not null;default:0;index:idx_videos_likes_count_id,priority:1,sort:desc" json:"likes_count"`
	CommentCount int64 `gorm:"not null;default:0"                                                    json:"comment_count"`
	Popularity   int64 `gorm:"not null;default:0;index:idx_videos_popularity_time_id,priority:1,sort:desc" json:"popularity"` // 热度
}

// PlayRecord 播放流水:每次播放插一条,只增不改。
// 与 videos.play_count 分工:后者是聚合总数,读列表直接用;本表是原始明细,
// 用于完播率分析、防刷风控、后续推荐的行为数据。纯追加表,无 UpdatedAt,不软删除
type PlayRecord struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"                      json:"id"`
	UserID    int64     `gorm:"index:idx_play_user_time,priority:1;not null"  json:"user_id"` // 0 表示未登录游客
	VideoID   int64     `gorm:"not null"                                      json:"video_id"`
	AuthorID  int64     `gorm:"not null"                                      json:"author_id"` // 冗余,便于按作者聚合免 JOIN
	Watched   int       `gorm:"not null;default:0"                            json:"watched"`   // 实际观看秒数
	Duration  int       `gorm:"not null;default:0"                            json:"duration"`  // 冗余,用于算完播率
	IP        string    `gorm:"type:varchar(45)"                              json:"ip"`        // IPv6 最长 45 字符
	CreatedAt time.Time `gorm:"index:idx_play_user_time,priority:2,sort:desc" json:"created_at"`
}
