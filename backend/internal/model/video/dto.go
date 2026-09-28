package video

import "time"

// UpdateVideoReq 编辑视频元数据(PUT /videos/:id)。
//
// 不含 PlayURL:播放地址由服务端在合并分片时推导,让客户端传就等于允许它
// 指向别人的文件或外站资源。
//
// CoverURL 不加 binding:"url" —— 它存的是站内相对路径(/covers/xx.png),
// url 规则会直接判非法;格式改由 service 层校验前缀
type UpdateVideoReq struct {
	Title       string `json:"title"       binding:"required,max=255"`
	Description string `json:"description" binding:"max=1000"`
	CoverURL    string `json:"cover_url"   binding:"omitempty,max=255"` // 留空表示不改动现有封面
}

// PlayReportReq 播放上报请求(POST /videos/:id/play)。
//
// 不含 video_id:ID 走路径参数。路径和 body 都能带的话,两者不一致时以谁为准
// 就成了 bug 温床 —— 详情 / 编辑 / 发布也都是从路径取 ID。
//
// 两个字段都要 min=0:不卡的话前端传负数能过,完播率会算出负值
type PlayReportReq struct {
	Watched  int `json:"watched"  binding:"min=0"` // 实际观看秒数
	Duration int `json:"duration" binding:"min=0"` // 总时长(秒),用于算完播率
}

// VideoResp 视频视图
// 显式列出字段:entity 里的 DeletedAt 与 Popularity(内部排序分)不对外暴露
type VideoResp struct {
	ID           int64     `json:"id"`
	AuthorID     int64     `json:"author_id"`
	Username     string    `json:"username"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	PlayURL      string    `json:"play_url"`
	CoverURL     string    `json:"cover_url"`
	CreatedAt    time.Time `json:"created_at"`
	Status       int8      `json:"status"` // 状态(0 转码中 / 1 已发布 / 2 转码失败 / 3 已下架 / 4 草稿)
	PlayCount    int64     `json:"play_count"`
	LikesCount   int64     `json:"likes_count"`
	CommentCount int64     `json:"comment_count"`
}

// ListLatestReq 最新流查询参数(GET /videos/latest)。
// 用 cursor 而非 offset —— offset 在翻页期间有新视频发布会漏条/重复,
// cursor 锚住上一页最后一条的发布时间,不受新增影响
type ListLatestReq struct {
	// Limit 每页条数。0 由 service 补默认值;max=100 卡住「一次拉全表」
	Limit int `form:"limit" binding:"omitempty,min=1,max=100"`
	// Cursor 上一页返回的 next_cursor(Unix 秒),首页不传。
	// 秒级精度会漏掉同一秒内尚未返回的视频;当前业务接受这个精度损失。
	Cursor int64 `form:"cursor" binding:"omitempty,min=0"`
}

// ListLatestResp 最新流响应。
//
// 不带 has_more:next_cursor 为 0 就等价于「没有更多」,
// 两个字段表达同一件事只会让前端不知道该信哪个
type ListLatestResp struct {
	// Items 视频卡片,按发布时间倒序。
	// service 必须保证它非 nil(空页返回 [] 而不是 null),前端就不用判空
	Items []VideoResp `json:"items"`
	// NextCursor 下一页游标(Unix 秒);0 表示没有更多
	NextCursor int64 `json:"next_cursor"`
}

type InitChunkUploadRequest struct {
	// max=255 对齐 videos.title 的 VARCHAR(255)(该值会当草稿标题写进库):不在这拦住,
	// 用户传完 1GB 才会在建视频行时撞上「value too long」。varchar 与 validator 的 max 都按 rune 计数
	Filename string `json:"filename" binding:"required,max=255"`
	FileSize int64  `json:"file_size" binding:"required,min=1"` // 单位 Byte
	// FileHash 客户端算好的 sha256 hex,不做秒传,只用于合并后校验完整性:服务端会再算一遍比对,对不上就丢弃整个上传。
	// hexadecimal 不能省 —— validator 的 len 数 rune 不数 byte,len=64 会放过 64 个中文字符,base64 规则也认不出 hex
	FileHash string `json:"file_hash" binding:"required,len=64,hexadecimal"`
}

// InitChunkUploadResp 分片上传初始化响应。不做秒传,没有「文件已存在、不用传」这条分支,每次 init 都返回新会话
type InitChunkUploadResp struct {
	UploadID    string `json:"upload_id"`          // 后续分片与合并都要带上
	ChunkSize   int64  `json:"chunk_size"`         // 每片字节数由服务端定;最后一片可能更小
	TotalChunks int    `json:"total_chunks"`       // 分片总数 = ceil(file_size / chunk_size)
	Uploaded    []int  `json:"uploaded,omitempty"` // 已存在的分片序号(0 起),用于断点续传
}

// UploadChunkRequest 描述一次分片上传请求。元数据走 multipart 表单字段,必须用 form tag ——
// gin 解析 multipart 读的是 form 不是 json,写成 json tag 会绑定不到、UploadID 恒为空,再被 required 判成"参数无效"。
// ChunkIndex 用 int 且不加 required:0 是合法序号,加了会把第 0 片误判成"字段缺失"
type UploadChunkRequest struct {
	UploadID   string `form:"upload_id"   binding:"required"`
	ChunkIndex int    `form:"chunk_index" binding:"min=0"` // 分片序号,从 0 开始
}

// UploadChunkResp 返回分片上传结果和当前上传进度。
type UploadChunkResp struct {
	UploadID      string `json:"upload_id"`
	ChunkIndex    int    `json:"chunk_index"`
	Uploaded      bool   `json:"uploaded"`
	Completed     bool   `json:"completed"` // 收齐后可以调 CompleteChunkUpload
	UploadedCount int    `json:"uploaded_count"`
	TotalChunks   int    `json:"total_chunks"`
}

// CompleteChunkUploadReq 合并分片、完成上传。只带 upload_id:文件地址由服务端推导,
// 不接受客户端传 —— 否则客户端能指向任意 URL,包括别人的文件和外站资源
type CompleteChunkUploadReq struct {
	UploadID string `json:"upload_id" binding:"required"` // init 返回的会话 ID
}

// CompleteChunkUploadResp 合并结果。返回 video_id 而非 URL:文件已落盘并建成草稿视频,
// 客户端拿 ID 去调编辑接口补标题和封面,全程不接触存储路径
type CompleteChunkUploadResp struct {
	VideoID  int64 `json:"video_id"`
	FileSize int64 `json:"file_size"` // 合并后的实际字节数,与声明对不上说明合并有问题
}
