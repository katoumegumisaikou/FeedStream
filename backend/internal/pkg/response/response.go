// Package response 提供统一的 HTTP 响应封装。
package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"feed-system/internal/pkg/errs"
)

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

// Error 返回业务错误响应;ServiceErr 用其 code/msg,其他 error 一律 "未知错误",防内部信息泄露
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

// SetTokenCookies 把 access / refresh token 设为 HttpOnly cookie,不再放进响应 body。
// HttpOnly 防前端 JS 读取(XSS 盗取);Secure=false 仅限开发,生产必须改 true(需 HTTPS);
// MaxAge 与 token 有效期一致(access 2h,refresh 30d)。
func SetTokenCookies(c *gin.Context, accessToken, refreshToken string) {
	const (
		accessMaxAge  = int(2 * time.Hour / time.Second)
		refreshMaxAge = int(30 * 24 * time.Hour / time.Second)
	)
	c.SetCookie("access_token", accessToken, accessMaxAge, "/", "", false, true)
	c.SetCookie("refresh_token", refreshToken, refreshMaxAge, "/", "", false, true)
}

// ClearTokenCookies 置空 cookie(登出用);MaxAge=-1 让浏览器立即删除
func ClearTokenCookies(c *gin.Context) {
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)
}
