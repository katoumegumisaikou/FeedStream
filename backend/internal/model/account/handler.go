package account

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"feed-system/internal/middleware"
	"feed-system/internal/pkg/errs"
	"feed-system/internal/pkg/response"
)

// AccountHandler 用户账号 HTTP handler
// 负责参数解析、调用 service、返回 JSON 响应
type AccountHandler struct {
	svc *AccountService
}

// NewAccountHandler 构造 AccountHandler
// 依赖 *AccountService,后续若有需要可改为接口注入便于单测
func NewAccountHandler(svc *AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

// Register 用户注册
// POST /api/v1/auth/register
func (h *AccountHandler) Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		response.Error(c, errs.ErrInvalidParam)
		return
	}

	token, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SetTokenCookies(c, token.AccessToken, token.RefreshToken)
	response.OK(c, gin.H{"logged_in": true})
}

// Login 用户登录
// POST /api/v1/auth/login
func (h *AccountHandler) Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		response.Error(c, errs.ErrInvalidParam)
		return
	}

	token, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SetTokenCookies(c, token.AccessToken, token.RefreshToken)
	response.OK(c, gin.H{"logged_in": true})
}

// Logout 用户登出
// POST /api/v1/auth/logout
// 从 cookie 读 token,服务端加入黑名单 + 浏览器侧清除 cookie
func (h *AccountHandler) Logout(c *gin.Context) {
	accessToken, _ := c.Cookie("access_token")
	refreshToken, _ := c.Cookie("refresh_token")

	if err := h.svc.Logout(c.Request.Context(), accessToken, refreshToken); err != nil {
		response.Error(c, err)
		return
	}
	response.ClearTokenCookies(c)
	response.OK(c, gin.H{"logged_out": true})
}

// GetProfile 获取指定用户资料
// GET /api/v1/users/:id
func (h *AccountHandler) GetProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrInvalidParam.WithMsg("用户 ID 无效"))
		return
	}
	user, err := h.svc.GetProfile(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

// GetMyProfile 获取当前登录用户资料
// GET /api/v1/users/me
func (h *AccountHandler) GetMyProfile(c *gin.Context) {
	user, err := h.svc.GetProfile(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

// UpdateProfile 更新当前登录用户资料
// PUT /api/v1/users/me
func (h *AccountHandler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileReq
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		response.Error(c, errs.ErrInvalidParam)
		return
	}
	user, err := h.svc.UpdateProfile(c.Request.Context(), middleware.UserID(c), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

// ChangePassword 修改密码
// PUT /api/v1/users/me/password
// 不需要 JWT 鉴权(走 SMS 验证码流程)
func (h *AccountHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordReq
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		response.Error(c, errs.ErrInvalidParam)
		return
	}

	token, err := h.svc.ChangePassword(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SetTokenCookies(c, token.AccessToken, token.RefreshToken)
	response.OK(c, nil)
}

// SendSmsCode 发送短信验证码(改密、注册等场景的前置步骤)
// POST /api/v1/auth/sms-code
// 不需要 JWT 鉴权
func (h *AccountHandler) SendSmsCode(c *gin.Context) {
	var req SendSmsCodeReq
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		response.Error(c, errs.ErrInvalidParam)
		return
	}
	code, err := h.svc.SendSmsCode(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	// dev 模式:把验证码放进响应方便测试(由 service.devMode 控制)
	// 生产环境:service.devMode=false,只返回 sent=true

	if code != "" {
		response.OK(c, gin.H{"sms_code": code})
	}

}

func (h *AccountHandler) Refresh(c *gin.Context) {
	// 直接从 cookie 读 refresh_token(前端不传)
	refreshToken, _ := c.Cookie("refresh_token")

	token, err := h.svc.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		response.Error(c, err)
		return
	}
	// 新的 token 重新写回 cookie(覆盖旧的,浏览器无感知)
	response.SetTokenCookies(c, token.AccessToken, token.RefreshToken)
	response.OK(c, nil) // 新 token 已在 cookie 里,body 不需要重复
}

func (h *AccountHandler) UploadAvatar(c *gin.Context) {
	// 必须用 middleware.UserID:它读的 key 是 "userID"(驼峰),
	// 手写 c.GetInt64("user_id") 会取不到值,恒为 0 → 永远 401
	userID := middleware.UserID(c)
	if userID == 0 {
		response.Error(c, errs.ErrUnauthorized)
		return
	}
	// ⚠️ 必须先判 err 再用 fileheader:FormFile 失败时返回 nil,
	//    先访问 fileheader.Size 会空指针 panic
	fileheader, err := c.FormFile("avatar")
	if err != nil {
		response.Error(c, errs.ErrInvalidParam.WithMsg("请选择要上传的头像文件"))
		return
	}

	if err := h.svc.UploadAvatar(c.Request.Context(), fileheader, userID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
