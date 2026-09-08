// Package response 提供统一的 HTTP 响应封装。
package response

import (
	"net/http"

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
func Error(c *gin.Context, e errs.ServiceErr) {
	c.JSON(http.StatusOK, gin.H{
		"code": e.Code,
		"msg":  e.Msg,
	})
}
