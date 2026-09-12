package account

import "time"

// RegisterReq 用户注册请求
type RegisterReq struct {
	UserName string  `json:"user_name" binding:"required,min=3,max=32"` // 用户名(3-32 字符)
	Password string  `json:"password" binding:"required,min=8,max=16"`  // 密码(8-16 字符,须与 password.Strong 的 MinLen/MaxLen 保持一致)
	Phone    string  `json:"phone" binding:"required,len=11"`           // 手机号(11 位)
	Email    *string `json:"email,omitempty" binding:"omitempty,email"` // 邮箱(可选,需合法格式)
}

// TokenResp 通用 token 响应(注册/登录/刷新 token 共用)
type TokenResp struct {
	AccessToken  string `json:"access_token"`  // 访问令牌(短寿命,默认 2 小时,用于 API 鉴权)
	RefreshToken string `json:"refresh_token"` // 刷新令牌(长寿命,默认 30 天,用于续期 access_token)
}

// LoginReq 登录请求
type LoginReq struct {
	Phone    string `json:"phone" binding:"required,len=11"` // 手机号(11 位)
	Password string `json:"password" binding:"required"`     // 密码(明文,后端用 bcrypt 校验)
}

// UpdateProfileReq 更新当前用户资料
// 用户名、手机号、密码不允许从这里改(走单独接口,需校验旧密码/验证码)
type UpdateProfileReq struct {
	AvatarURL *string `json:"avatar_url,omitempty" binding:"omitempty,url"`   // 头像 URL(可选)
	Email     *string `json:"email,omitempty"      binding:"omitempty,email"` // 邮箱(可选,需合法格式)
}

// UserResp 暴露给前端的用户视图
// 显式列出字段,防止 entity 改字段时意外泄露(Password / DeletedAt 永不暴露)
type UserResp struct {
	ID          int64      `json:"id"`                      // 主键 ID
	UserName    string     `json:"user_name"`               // 用户名
	Phone       string     `json:"phone"`                   // 手机号
	Email       *string    `json:"email,omitempty"`         // 邮箱(可空)
	AvatarURL   string     `json:"avatar_url,omitempty"`    // 头像 URL(可空)
	CreatedAt   time.Time  `json:"created_at"`              // 注册时间
	LastLoginAt *time.Time `json:"last_login_at,omitempty"` // 最近登录时间(可空)
}

// ChangePasswordReq 修改密码请求
// 走短信验证码流程:用户先请求发送验证码到手机,再提交验证码 + 新密码完成修改
// 适用于"忘记旧密码"或"高敏感场景需二次验证"的场景
type ChangePasswordReq struct {
	Phone    string `json:"phone"            binding:"required,len=11"`      // 手机号(11 位)
	Password string `json:"password"         binding:"required,min=8,max=16"` // 新密码(8-16 字符,后端重新 bcrypt)
	SmsCode  string `json:"sms_code"         binding:"required,len=6"`       // 短信验证码(6 位数字)
}

// SendSmsCodeReq 发送短信验证码请求
// 改密、注册、找回密码等场景需要时,先调此接口给手机发验证码
type SendSmsCodeReq struct {
	Phone string `json:"phone" binding:"required,len=11"` // 手机号(11 位)
}
