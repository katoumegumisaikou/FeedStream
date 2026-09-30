package feed

import (
	"time"
)

// ListLatestReq 最新视频流查询参数(GET /videos/latest)。
type ListLatestReq struct {
	// Limit 每页条数。0 由 service 补默认值。
	Limit int `form:"limit" binding:"omitempty,min=1,max=100"`
	// Cursor 上一页返回的 next_cursor(Unix 秒),首页不传。
	// 秒级精度会漏掉同一秒内尚未返回的视频;当前业务接受这个精度损失。
	Cursor int64 `form:"cursor" binding:"omitempty,min=0"`
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

// ListLatestResp 最新视频流响应。
type ListLatestResp struct {
	// Items service 保证非 nil,空页也返回 [] 而不是 null。
	Items []FeedItem `json:"items"`
	// NextCursor 下一页游标(Unix 秒)。只在确实还有下一页时才给,到底了留 0 ——
	// 前端可以直接拿它决定「加载更多」显不显示,不用再多请求一次空页来确认
	NextCursor int64 `json:"next_cursor"`
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
