// Package response 提供统一的 HTTP 响应封装。
package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"feed-system/internal/pkg/errs"
)

// OK 返回成功响应
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

// Error 返回业务错误响应
// ServiceErr 用它的 code/msg,其他 error 一律返回 "未知错误"(防内部信息泄露)
func Error(c *gin.Context, e error) {
	if se, ok := e.(errs.ServiceErr); ok {
		c.JSON(http.StatusOK, gin.H{
			"code": se.Code,
			"msg":  se.Error(),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"code": errs.Failed,
			"msg":  "未知错误",
		})
	}
}

// SetTokenCookies 把 access / refresh token 设为 HttpOnly cookie
//   - HttpOnly=true: 前端 JS 读不到,防 XSS 盗取
//   - Secure=false: 开发环境 HTTP 也发,生产环境务必改成 true(需 HTTPS)
//   - Path=/:全路径有效
//   - MaxAge 与 token 有效期一致(access 2h,refresh 30d)
//
// 用法:登录/注册成功后调用此函数,不再把 token 放进响应 body
func SetTokenCookies(c *gin.Context, accessToken, refreshToken string) {
	const (
		accessMaxAge  = int(2 * time.Hour / time.Second)      // 2 小时
		refreshMaxAge = int(30 * 24 * time.Hour / time.Second) // 30 天
	)
	c.SetCookie("access_token", accessToken, accessMaxAge, "/", "", false, true)
	c.SetCookie("refresh_token", refreshToken, refreshMaxAge, "/", "", false, true)
}

// ClearTokenCookies 把 access / refresh cookie 设为立即过期(登出用)
// MaxAge=-1 告诉浏览器立即删除
func ClearTokenCookies(c *gin.Context) {
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)
}