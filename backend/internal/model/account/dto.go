package account

import "time"

// RegisterReq 用户注册请求
type RegisterReq struct {
	UserName string  `json:"user_name" binding:"required,min=3,max=32"`
	Password string  `json:"password" binding:"required,min=8,max=16"` // 长度须与 password.Strong 的 MinLen/MaxLen 一致
	Phone    string  `json:"phone" binding:"required,len=11"`
	Email    *string `json:"email,omitempty" binding:"omitempty,email"`
}

// TokenResp 通用 token 响应(注册/登录/刷新共用)
type TokenResp struct {
	AccessToken  string `json:"access_token"`  // 默认寿命 2 小时,用于 API 鉴权
	RefreshToken string `json:"refresh_token"` // 默认寿命 30 天,用于续期 access_token
}

// LoginReq 登录请求
type LoginReq struct {
	Phone    string `json:"phone" binding:"required,len=11"`
	Password string `json:"password" binding:"required"`
}

// UpdateProfileReq 更新当前用户资料。
// 用户名、手机号、密码不能从这里改,走各自接口(需校验旧密码/验证码)
type UpdateProfileReq struct {
	AvatarURL *string `json:"avatar_url,omitempty" binding:"omitempty,url"`
	Email     *string `json:"email,omitempty"      binding:"omitempty,email"`
}

// UserResp 暴露给前端的用户视图。
// 显式列字段,防止 entity 改字段时意外泄露(Password / DeletedAt 永不暴露)
type UserResp struct {
	ID          int64      `json:"id"`
	UserName    string     `json:"user_name"`
	Phone       string     `json:"phone"`
	Email       *string    `json:"email,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// PublicUserResp 公开资料视图(查他人资料用)。
// 与 UserResp 的差别是没有 Phone / Email / LastLoginAt ——
// 后者的活跃时段和作息也是隐私
type PublicUserResp struct {
	ID        int64     `json:"id"`
	UserName  string    `json:"user_name"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ChangePasswordReq 修改密码请求。
// 走短信验证码流程:先发码到手机,再提交验证码 + 新密码
type ChangePasswordReq struct {
	Phone    string `json:"phone"            binding:"required,len=11"`
	Password string `json:"password"         binding:"required,min=8,max=16"`
	SmsCode  string `json:"sms_code"         binding:"required,len=6"`
}

// SendSmsCodeReq 发送短信验证码请求
type SendSmsCodeReq struct {
	Phone string `json:"phone" binding:"required,len=11"`
}
