package feed

import (
	"time"
)

// ListLatestReq 最新视频流查询参数(GET /videos/latest)。
//
// 游标是复合的 (created_at, id):只带时间戳的话,同一时刻发布的视频没有确定顺序,
// 翻页会漏条或重复。带上 id 兜底,边界就从「某个时刻」变成「某条记录之前」。
//
// 时间戳用微秒,和 videos.created_at(TIMESTAMP,微秒精度)齐平 —— 必须能精确还原
// 库里的值,否则 id 那个兜底条件的等号永远不成立,等于没加。
// 微秒时间戳现在约 1.79e15,离 JS 的 MAX_SAFE_INTEGER(9.007e15)还有 5 倍余量
type ListLatestReq struct {
	// Limit 每页条数。0 由 service 补默认值。
	Limit int `form:"limit" binding:"omitempty,min=1,max=100"`
	// CursorCreatedAt 上一页最后一条的 created_at(Unix 微秒)。首页不传
	CursorCreatedAt *int64 `form:"cursor_created_at" binding:"omitempty,min=0"`
	// CursorVideoID 上一页最后一条的视频 ID。首页不传;必须和 CursorCreatedAt 同时给
	CursorVideoID *int64 `form:"cursor_video_id" binding:"omitempty,min=1"`
}

// FeedItem 最新视频流中的视频卡片。
type FeedItem struct {
	ID           int64     `json:"id"`
	AuthorID     int64     `json:"author_id"`
	Username     string    `json:"username"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	PlayURL      string    `json:"play_url"`
	CoverURL     string    `json:"cover_url"`
	CreatedAt    time.Time `json:"created_at"`
	Status       int8      `json:"status"`
	PlayCount    int64     `json:"play_count"`
	LikesCount   int64     `json:"likes_count"`
	CommentCount int64     `json:"comment_count"`
	IsLike       *bool     `json:"is_like,omitempty"` // nil 表示未登录或未计算;非 nil 时表示当前用户是否点赞
}

// FeedCursor 最新流的下一页游标。
//
// 两个字段必须一起带回来:只带 created_at 的话,同一微秒发布的视频会漏或重复
type FeedCursor struct {
	CreatedAt int64 `json:"created_at"` // Unix 微秒
	VideoID   int64 `json:"video_id"`
}

// ListLatestResp 最新视频流响应。
type ListLatestResp struct {
	// Items service 保证非 nil,空页也返回 [] 而不是 null。
	Items []FeedItem `json:"items"`
	// NextCursor 下一页游标。只在确实还有下一页时才给,到底了是 null ——
	// 前端可以直接拿它决定「加载更多」显不显示,不用再多请求一次空页来确认
	NextCursor *FeedCursor `json:"next_cursor,omitempty"`
}

// ListLikeReq 按点赞数排序的视频流请求。
// 首页不传游标;翻页时两个游标字段需同时传。
type ListLikeReq struct {
	Limit            int    `form:"limit" binding:"omitempty,min=1,max=100"`
	CursorLikesCount *int64 `form:"cursor_likes_count" binding:"omitempty,min=0"`
	CursorVideoID    *int64 `form:"cursor_video_id" binding:"omitempty,min=1"`
}

// LikeCursor 是视频流的下一页游标。
type LikeCursor struct {
	LikesCount int64 `json:"likes_count"`
	VideoID    int64 `json:"video_id"`
}

// ListLikeResp 按点赞数排序的视频流响应。
type ListLikeResp struct {
	Items []FeedItem `json:"items"`
	// nil 表示没有更多数据。
	NextCursor *LikeCursor `json:"next_cursor,omitempty"`
}
