package video

import "time"

// PublishVideoReq 发布视频请求
//
// PlayURL / CoverURL 不加 binding:"url":本项目静态资源存的是相对路径
// (头像就是 /avatars/<uid>/<uuid>.png),策略库的 url 校验会直接判为非法
type PublishVideoReq struct {
	Title       string `json:"title"       binding:"required,max=255"` // 标题
	Description string `json:"description" binding:"max=1000"`         // 简介(可空)
	PlayURL     string `json:"play_url"    binding:"required,max=255"` // 播放地址
	CoverURL    string `json:"cover_url"   binding:"required,max=255"` // 封面地址
}

// PlayReportReq 播放上报请求
type PlayReportReq struct {
	VideoID  int64 `json:"video_id" binding:"required"` // 被观看的视频 ID
	Watched  int   `json:"watched"`                     // 本次实际观看秒数
	Duration int   `json:"duration"`                    // 视频总时长(秒),用于算完播率
}

// VideoResp 视频视图
// 显式列出字段:entity 里的 DeletedAt 与 Popularity(内部排序分)不对外暴露
type VideoResp struct {
	ID           int64     `json:"id"`                    // 主键 ID
	AuthorID     int64     `json:"author_id"`             // 作者用户 ID
	Username     string    `json:"username"`              // 作者用户名
	AvatarURL    string    `json:"avatar_url,omitempty"`  // 作者头像(可空)
	Title        string    `json:"title"`                 // 标题
	Description  string    `json:"description,omitempty"` // 简介(可空)
	PlayURL      string    `json:"play_url"`              // 播放地址
	CoverURL     string    `json:"cover_url"`             // 封面地址
	CreatedAt    time.Time `json:"created_at"`            // 发布时间
	Status       int8      `json:"status"`                // 状态(0 转码中 / 1 已发布 / 2 转码失败 / 3 已下架)
	PlayCount    int64     `json:"play_count"`            // 播放数
	LikesCount   int64     `json:"likes_count"`           // 点赞数
	CommentCount int64     `json:"comment_count"`         // 评论数
}

type InitChunkUploadRequest struct {
	// max=255 对齐 videos.title 的 VARCHAR(255):这个值会当草稿标题写进库,
	// 不在 init 拦住的话,用户传完 1GB 才会在建视频行时撞上「value too long」,
	// 白传一场还看不出原因。varchar 和 validator 的 max 数的都是 rune,口径一致
	Filename string `json:"filename" binding:"required,max=255"`
	FileSize int64  `json:"file_size" binding:"required,min=1"` // 文件大小,单位为 Byte
	// FileHash 客户端算好的 sha256 hex。不做秒传,它只用于合并后校验完整性 ——
	// 服务端会自己再算一遍比对,对不上就丢弃整个上传
	//
	// hexadecimal 不能省:validator 的 len 数的是 rune 不是 byte,
	// 单靠 len=64 会放过 64 个中文字符;而且 base64 规则认不出 hex,别指望它
	FileHash string `json:"file_hash" binding:"required,len=64,hexadecimal"`
}

// InitChunkUploadResp 分片上传初始化响应
//
// 不做秒传,所以没有「文件已存在、不用传」这条分支 —— 每次 init 都返回一个上传会话
type InitChunkUploadResp struct {
	UploadID    string `json:"upload_id"`          // 上传会话 ID,后续分片与合并都要带上
	ChunkSize   int64  `json:"chunk_size"`         // 每片字节数,由服务端定;最后一片可能更小
	TotalChunks int    `json:"total_chunks"`       // 分片总数 = ceil(file_size / chunk_size)
	Uploaded    []int  `json:"uploaded,omitempty"` // 已存在的分片序号(0 起),用于断点续传
}

// UploadChunkRequest 描述一次分片上传请求。
//
// 元数据走 multipart 的表单字段,所以必须用 form tag —— gin 解析 multipart 时
// 读的是 form 不是 json;写成 json tag 会绑定不到,UploadID 恒为空,
// 而 binding:"required" 会让每个请求都提示"参数无效"
//
// ChunkIndex 用 int 且不加 required:0 是合法的分片序号,
// 加 required 会把第 0 片误判成"字段缺失"
type UploadChunkRequest struct {
	UploadID   string `form:"upload_id"   binding:"required"` // 上传会话 ID
	ChunkIndex int    `form:"chunk_index" binding:"min=0"`    // 分片序号,从 0 开始
}

// UploadChunkResp 返回分片上传结果和当前上传进度。
type UploadChunkResp struct {
	UploadID      string `json:"upload_id"`      // 上传会话 ID
	ChunkIndex    int    `json:"chunk_index"`    // 已处理的分片序号
	Uploaded      bool   `json:"uploaded"`       // 当前分片是否已成功保存
	Completed     bool   `json:"completed"`      // 分片是否已收齐,收齐后可以调 CompleteChunkUpload
	UploadedCount int    `json:"uploaded_count"` // 已上传分片数量
	TotalChunks   int    `json:"total_chunks"`   // 分片总数
}

// CompleteChunkUploadReq 合并分片、完成上传
//
// 只带 upload_id:客户端在这个阶段只有它。文件地址由服务端自己推导,
// 不接受客户端传 —— 否则客户端可以指向任意 URL,包括别人的文件和外站资源。
type CompleteChunkUploadReq struct {
	UploadID string `json:"upload_id" binding:"required"` // init 返回的会话 ID
}

// CompleteChunkUploadResp 合并结果
//
// 返回 video_id 而不是 URL:文件已经落盘并建成一条草稿视频,
// 客户端拿 ID 去调编辑接口补标题和封面。客户端全程不接触存储路径。
type CompleteChunkUploadResp struct {
	VideoID  int64 `json:"video_id"`  // 新建的草稿视频 ID
	FileSize int64 `json:"file_size"` // 合并后的实际字节数,与声明对不上说明合并有问题
}
