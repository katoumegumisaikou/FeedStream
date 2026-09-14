package video

import (
	"time"

	"gorm.io/gorm"
)

// 视频状态
const (
	StatusTranscoding     int8 = iota // 0 转码中
	StatusPublished                   // 1 已发布
	StatusTranscodeFailed             // 2 转码失败
	StatusRemoved                     // 3 已下架
)

type Video struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"   json:"id"`                    // 主键 ID
	AuthorID    int64  `gorm:"index;not null"             json:"author_id"`             // 作者用户 ID
	Username    string `gorm:"type:varchar(255);not null" json:"username"`              // 作者用户名(冗余,列表免 JOIN)
	AvatarURL   string `gorm:"type:varchar(512)"          json:"avatar_url,omitempty"`  // 作者头像(冗余,可空)
	Title       string `gorm:"type:varchar(255);not null" json:"title"`                 // 视频标题
	Description string `gorm:"type:varchar(1000)"         json:"description,omitempty"` // 视频简介(可空)
	PlayURL     string `gorm:"type:varchar(255);not null" json:"play_url"`              // 播放地址
	CoverURL    string `gorm:"type:varchar(255);not null" json:"cover_url"`             // 封面地址

	// CreatedAt / UpdatedAt 是 GORM 的约定名,按名字自动识别并填充,不需要 autoCreateTime tag
	CreatedAt time.Time      `gorm:"index:idx_videos_create_time,sort:desc;index:idx_videos_popularity_time_id,priority:2,sort:desc" json:"created_at"` // 发布时间
	UpdatedAt time.Time      `                                                                                                       json:"updated_at"` // 更新时间
	DeletedAt gorm.DeletedAt `                                                                                                       json:"-"`          // 软删除标记,不暴露给前端

	Status       int8  `gorm:"not null;default:0;index"                                              json:"status"`           // 状态(0 转码中 / 1 已发布 / 2 转码失败 / 3 已下架)
	PlayCount    int64 `gorm:"not null;default:0"                                                    json:"play_count"`       // 播放数
	LikesCount   int64 `gorm:"not null;default:0;index:idx_videos_likes_count_id,priority:1,sort:desc" json:"likes_count"`    // 点赞数
	CommentCount int64 `gorm:"not null;default:0"                                                    json:"comment_count"`    // 评论数
	Popularity   int64 `gorm:"not null;default:0;index:idx_videos_popularity_time_id,priority:1,sort:desc" json:"popularity"` // 热度
}

// PlayRecord 播放流水:每次播放插入一条,只增不改。
//
// 与 videos.play_count 的分工:
//   - videos.play_count 是聚合后的总数,读列表时直接用
//   - 这张表是原始明细,用于完播率分析、防刷风控、后续推荐系统的行为数据
//
// 因为是纯追加表,没有 UpdatedAt,也不做软删除
type PlayRecord struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"                      json:"id"`         // 主键 ID
	UserID    int64     `gorm:"index:idx_play_user_time,priority:1;not null"  json:"user_id"`    // 观看用户 ID(0 表示未登录游客)
	VideoID   int64     `gorm:"not null"                                      json:"video_id"`   // 被观看的视频 ID
	AuthorID  int64     `gorm:"not null"                                      json:"author_id"`  // 视频作者 ID(冗余,便于按作者聚合,免 JOIN)
	Watched   int       `gorm:"not null;default:0"                            json:"watched"`    // 本次实际观看秒数
	Duration  int       `gorm:"not null;default:0"                            json:"duration"`   // 视频总时长(冗余,用于算完播率)
	IP        string    `gorm:"type:varchar(45)"                              json:"ip"`         // 来源 IP(IPv6 最长 45 字符)
	CreatedAt time.Time `gorm:"index:idx_play_user_time,priority:2,sort:desc" json:"created_at"` // 播放时间
}
