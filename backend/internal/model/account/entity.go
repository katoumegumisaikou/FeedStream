package account

import (
	"time"

	"gorm.io/gorm"
)

// User 用户账号实体
type User struct {
	ID          int64          `gorm:"primaryKey;autoIncrement"     json:"id"`            // 主键 ID
	UserName    string         `gorm:"size:64;not null"             json:"user_name"`     // 用户名
	Password    string         `gorm:"size:64;not null"             json:"-"`             // 密码(bcrypt 哈希,严禁泄露,json:"-" 强制忽略)
	Phone       string         `gorm:"size:20;uniqueIndex;not null" json:"phone"`        // 手机号(唯一)
	Email       *string        `gorm:"size:128;uniqueIndex"         json:"email,omitempty"` // 邮箱(可空,唯一)
	AvatarURL   string         `gorm:"size:512"                     json:"avatar_url,omitempty"` // 头像 URL(可空)
	Version     int64          `gorm:"default:1"                    json:"-"`             // 版本号(改密码/注销时+1,用于让旧 JWT 失效)
	CreatedAt   time.Time      `                                  json:"created_at"`    // 注册时间
	LastLoginAt *time.Time     `                                  json:"last_login_at,omitempty"` // 最近登录时间(可空)
	DeletedAt   gorm.DeletedAt `                                  json:"-"`             // 软删除标记,不暴露给前端
}
