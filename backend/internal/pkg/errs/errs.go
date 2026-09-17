// Package errs 定义业务错误类型与全站通用错误码。
//
// 通用错误码只作"分类",具体描述由 service 层填 Msg 后返回;
// 内部错误(DB/缓存/第三方)用 errors.New,仅记日志,不暴露给前端。
package errs

import (
	"errors"
	"net/http"
)

// ServiceErr 业务错误。Code 是分类,Msg 是具体描述
type ServiceErr struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Error 实现标准 error 接口
func (e ServiceErr) Error() string {
	return e.Msg
}

// Is 支持 errors.Is:两个 ServiceErr 按 Code 判等
func (e ServiceErr) Is(target error) bool {
	var t ServiceErr
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// WithMsg 保留 Code、替换 Msg。用法:errs.ErrConflict.WithMsg("手机号已被注册")
func (e ServiceErr) WithMsg(msg string) ServiceErr {
	return ServiceErr{Code: e.Code, Msg: msg}
}

// HTTPStatus 业务码对应的 HTTP 状态码
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

// 通用错误码(具体 msg 由调用方填)
var (
	Success         = ServiceErr{Code: 0, Msg: "成功"}
	Failed          = ServiceErr{Code: 1, Msg: "操作失败"} // 非 ServiceErr 错误的兜底
	ErrInvalidParam = ServiceErr{Code: 10001, Msg: "参数无效"}
	ErrUnauthorized = ServiceErr{Code: 10002, Msg: "未登录"}
	ErrForbidden    = ServiceErr{Code: 10003, Msg: "无权限"}
	ErrNotFound     = ServiceErr{Code: 10004, Msg: "资源不存在"}
	ErrInternal     = ServiceErr{Code: 10005, Msg: "服务器内部错误"}
	ErrTooFrequent  = ServiceErr{Code: 10006, Msg: "操作过于频繁"}
	ErrConflict     = ServiceErr{Code: 10007, Msg: "资源冲突"}
)

// 内部错误:只记日志,不暴露给前端(service 层捕获后统一返回 ErrInternal)
var (
	ErrInternalDB         = errors.New("database error")
	ErrInternalCache      = errors.New("cache error")
	ErrInternalDownstream = errors.New("downstream error")
)
