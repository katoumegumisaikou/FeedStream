// Package errs 定义业务错误类型与全站业务错误码。
package errs

import (
	"errors"
	"net/http"
)

// ServiceErr 业务错误(实现标准 error 接口)
type ServiceErr struct {
	Code int    `json:"code"` // 业务数字码
	Msg  string `json:"msg"`  // 用户可读的消息
}

// New 构造业务错误
func New(code int, msg string) ServiceErr {
	return ServiceErr{Code: code, Msg: msg}
}

// Error 实现标准 error 接口,返回 Msg
func (e ServiceErr) Error() string {
	return e.Msg
}

// Is 实现 errors.Is 支持:按 Code 判定两个 ServiceErr 是否等价
// 用法:errors.Is(err, errs.ErrUserNotFound)
func (e ServiceErr) Is(target error) bool {
	var t ServiceErr
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// HTTPStatus 返回业务码对应的 HTTP 状态码
// 0 → 200;通用错误按定义映射;业务错误默认 400
func (e ServiceErr) HTTPStatus() int {
	switch e.Code {
	case 0:
		return http.StatusOK
	case ErrInvalidParam.Code:
		return http.StatusBadRequest
	case ErrUnauthorized.Code:
		return http.StatusUnauthorized
	case ErrForbidden.Code:
		return http.StatusForbidden
	case ErrNotFound.Code:
		return http.StatusNotFound
	case ErrInternal.Code:
		return http.StatusInternalServerError
	case ErrTooFrequent.Code:
		return http.StatusTooManyRequests
	case ErrConflict.Code:
		return http.StatusConflict
	default:
		// 11xxx 及以上业务码默认按客户端错误处理
		return http.StatusBadRequest
	}
}

// As 从 error 链中提取 ServiceErr
// 用法:e, ok := errs.As(err)
func As(err error) (ServiceErr, bool) {
	var e ServiceErr
	if errors.As(err, &e) {
		return e, true
	}
	return ServiceErr{}, false
}

// ============================================================
// 通用码(0 成功,1xxxx 通用错误)
// ============================================================
var (
	Success         = New(0, "成功")
	Failed          = New(1, "操作失败")
	ErrInvalidParam = New(10001, "参数无效")
	ErrUnauthorized = New(10002, "未登录或登录已过期")
	ErrForbidden    = New(10003, "无权限访问")
	ErrNotFound     = New(10004, "资源不存在")
	ErrInternal     = New(10005, "服务器内部错误")
	ErrTooFrequent  = New(10006, "操作过于频繁,请稍后再试")
	ErrConflict     = New(10007, "资源冲突")
	ErrDeprecated   = New(10008, "接口已废弃")
)

// ============================================================
// 用户 / 账号(11xxx)
// ============================================================
var (
	ErrUserNotFound       = New(11001, "用户不存在")
	ErrUserAlreadyExists  = New(11002, "用户已存在")
	ErrInvalidPhone       = New(11003, "手机号格式错误")
	ErrInvalidEmail       = New(11004, "邮箱格式错误")
	ErrInvalidPassword    = New(11005, "密码错误")
	ErrInvalidCredentials = New(11006, "用户名或密码错误")
	ErrPhoneTaken         = New(11007, "手机号已被注册")
	ErrEmailTaken         = New(11008, "邮箱已被注册")
	ErrUserNameTaken      = New(11009, "用户名已被占用")
	ErrWeakPassword       = New(11010, "密码强度不足(至少 8 位,含字母数字)")
	ErrUserDisabled       = New(11011, "账号已被禁用")
	ErrUserBanned         = New(11012, "账号已被封禁")
	ErrTokenInvalid       = New(11013, "token 无效")
	ErrTokenExpired       = New(11014, "token 已过期")
	ErrRefreshTokenFailed = New(11015, "刷新 token 失败")
	ErrCaptchaInvalid     = New(11016, "验证码错误")
	ErrSMSCodeInvalid     = New(11017, "短信验证码错误")
	ErrSMSCodeExpired     = New(11018, "短信验证码已过期")
)

// ============================================================
// 视频(12xxx)
// ============================================================
var (
	ErrVideoNotFound        = New(12001, "视频不存在")
	ErrVideoPrivate         = New(12002, "视频为私享,无权观看")
	ErrVideoDeleted         = New(12003, "视频已删除")
	ErrVideoUnderReview     = New(12004, "视频审核中,暂不可用")
	ErrVideoRejected        = New(12005, "视频审核未通过")
	ErrVideoTooLarge        = New(12006, "视频文件超出大小限制")
	ErrVideoFormatInvalid   = New(12007, "视频格式不支持")
	ErrVideoTooShort        = New(12008, "视频时长过短")
	ErrVideoTooLong         = New(12009, "视频时长超出限制")
	ErrVideoEncodingFailed  = New(12010, "视频转码失败")
	ErrVideoUploadFailed    = New(12011, "视频上传失败")
	ErrVideoEditForbidden   = New(12012, "无权编辑该视频")
	ErrVideoDeleteForbidden = New(12013, "无权删除该视频")
	ErrTitleEmpty           = New(12014, "视频标题不能为空")
	ErrTitleTooLong         = New(12015, "视频标题过长")
	ErrDescriptionTooLong   = New(12016, "视频简介过长")
)

// ============================================================
// 评论(13xxx)
// ============================================================
var (
	ErrCommentNotFound      = New(13001, "评论不存在")
	ErrCommentForbidden     = New(13002, "无权操作该评论")
	ErrCommentEmpty         = New(13003, "评论内容不能为空")
	ErrCommentTooLong       = New(13004, "评论内容超出长度限制")
	ErrCommentSpam          = New(13005, "评论包含违规内容")
	ErrCommentRootNotFound  = New(13006, "根评论不存在")
	ErrCommentParentInvalid = New(13007, "父评论无效")
	ErrCommentTooFast       = New(13008, "评论发送过于频繁")
	ErrCommentClosed        = New(13009, "该视频已关闭评论")
)

// ============================================================
// 点赞 / 收藏 / 投币(14xxx)
// ============================================================
var (
	ErrAlreadyLiked       = New(14001, "已点赞该视频")
	ErrNotLikedYet        = New(14002, "尚未点赞该视频")
	ErrAlreadyFavorited   = New(14003, "已收藏该视频")
	ErrNotFavoritedYet    = New(14004, "尚未收藏该视频")
	ErrAlreadyCoined      = New(14005, "已投币该视频")
	ErrInsufficientCoin   = New(14006, "硬币余额不足")
	ErrFavoriteFolderFull = New(14007, "收藏夹已满")
)

// ============================================================
// 关注(15xxx)
// ============================================================
var (
	ErrAlreadyFollowing   = New(15001, "已关注该用户")
	ErrNotFollowing       = New(15002, "未关注该用户")
	ErrCannotFollowSelf   = New(15003, "不能关注自己")
	ErrFollowLimitReached = New(15004, "关注数已达上限")
	ErrFansLimitReached   = New(15005, "粉丝数已达上限")
	ErrBlockedByUser      = New(15006, "已被对方拉黑")
)

// ============================================================
// 文件 / 上传(16xxx)
// ============================================================
var (
	ErrFileTooLarge      = New(16001, "文件超出大小限制")
	ErrFileTypeInvalid   = New(16002, "文件类型不支持")
	ErrFileCorrupted     = New(16003, "文件损坏")
	ErrUploadFailed      = New(16004, "文件上传失败")
	ErrUploadExpired     = New(16005, "上传链接已过期")
	ErrStorageQuotaFull  = New(16006, "存储空间已满")
	ErrCoverImageInvalid = New(16007, "封面图片无效")
)

// ============================================================
// 弹幕(17xxx)
// ============================================================
var (
	ErrDanmakuEmpty        = New(17001, "弹幕内容不能为空")
	ErrDanmakuTooLong      = New(17002, "弹幕内容超出长度限制")
	ErrDanmakuTooFast      = New(17003, "弹幕发送过于频繁")
	ErrDanmakuClosed       = New(17004, "该视频已关闭弹幕")
	ErrDanmakuSpam         = New(17005, "弹幕包含违规内容")
	ErrDanmakuColorInvalid = New(17006, "弹幕颜色无效")
)

// ============================================================
// 分类 / 标签(18xxx)
// ============================================================
var (
	ErrCategoryNotFound = New(18001, "分类不存在")
	ErrCategoryDisabled = New(18002, "分类已停用")
	ErrTagNotFound      = New(18003, "标签不存在")
	ErrTagTooLong       = New(18004, "标签名称过长")
	ErrTooManyTags      = New(18005, "标签数量超出限制")
)

// ============================================================
// 通知 / 消息(19xxx)
// ============================================================
var (
	ErrNotificationNotFound = New(19001, "通知不存在")
	ErrNotificationDisabled = New(19002, "通知已被禁用")
)

// ============================================================
// 支付 / 会员(2xxxx)
// ============================================================
var (
	ErrOrderNotFound       = New(20001, "订单不存在")
	ErrOrderAlreadyPaid    = New(20002, "订单已支付")
	ErrOrderExpired        = New(20003, "订单已过期")
	ErrPaymentFailed       = New(20004, "支付失败")
	ErrInsufficientBalance = New(20005, "余额不足")
	ErrMembershipExpired   = New(20006, "会员已过期")
	ErrMembershipActive    = New(20007, "已是会员")
	ErrRefundFailed        = New(20008, "退款失败")
)

// ============================================================
// 第三方服务(3xxxx)
// ============================================================
var (
	ErrThirdPartyTimeout     = New(30001, "第三方服务超时")
	ErrThirdPartyUnavailable = New(30002, "第三方服务暂不可用")
	ErrThirdPartyRejected    = New(30003, "第三方服务拒绝请求")
	ErrSMSGatewayFailed      = New(30004, "短信网关发送失败")
	ErrCDNUploadFailed       = New(30005, "CDN 上传失败")
)
