// Package errs 定义业务错误类型与全站通用错误码。
//
// 设计原则:
//   - 通用错误码作为"分类",具体业务错误由 service 层填 Msg 后返回
//   - 业务侧不需要为每种业务错误都定义一个常量,大幅减少维护成本
//   - 内部错误(数据库/缓存/第三方)用 errors.New,仅记日志,不暴露给前端
package errs

import (
	"errors"
	"net/http"
)

// ServiceErr 业务错误(实现标准 error 接口)
// Code 是大分类,Msg 是具体描述
type ServiceErr struct {
	Code int    `json:"code"` // 大类
	Msg  string `json:"msg"`  // 具体描述
}

// Error 实现标准 error 接口,返回 Msg
func (e ServiceErr) Error() string {
	return e.Msg
}

// Is 实现 errors.Is 支持:按 Code 判定两个 ServiceErr 是否等价
func (e ServiceErr) Is(target error) bool {
	var t ServiceErr
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// WithMsg 返回一个新的 ServiceErr,沿用当前 Code 但用传入的 Msg
// 用法:errs.ErrConflict.WithMsg("手机号已被注册")
func (e ServiceErr) WithMsg(msg string) ServiceErr {
	return ServiceErr{Code: e.Code, Msg: msg}
}

// HTTPStatus 返回业务码对应的 HTTP 状态码
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
		return http.StatusBadRequest
	}
}

// As 从 error 链中提取 ServiceErr
func As(err error) (ServiceErr, bool) {
	var e ServiceErr
	if errors.As(err, &e) {
		return e, true
	}
	return ServiceErr{}, false
}

// ============================================================
// 通用错误码(分类,具体 msg 由调用方填)
// ============================================================
var (
	Success         = ServiceErr{Code: 0, Msg: "成功"}
	Failed          = ServiceErr{Code: 1, Msg: "操作失败"} // 非 ServiceErr 错误的兜底(如 response 包收到的 raw error)
	ErrInvalidParam = ServiceErr{Code: 10001, Msg: "参数无效"}
	ErrUnauthorized = ServiceErr{Code: 10002, Msg: "未登录"}
	ErrForbidden    = ServiceErr{Code: 10003, Msg: "无权限"}
	ErrNotFound     = ServiceErr{Code: 10004, Msg: "资源不存在"}
	ErrInternal     = ServiceErr{Code: 10005, Msg: "服务器内部错误"}
	ErrTooFrequent  = ServiceErr{Code: 10006, Msg: "操作过于频繁"}
	ErrConflict     = ServiceErr{Code: 10007, Msg: "资源冲突"}
)

// ============================================================
// 内部错误(只给日志看,不给前端)
// service 层捕获后,记日志 + 返回 ErrInternal 给前端
// ============================================================
var (
	ErrInternalDB         = errors.New("database error")   // DB 出错
	ErrInternalCache      = errors.New("cache error")      // Redis 出错
	ErrInternalDownstream = errors.New("downstream error") // 第三方服务出错
)
