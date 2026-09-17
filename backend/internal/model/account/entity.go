package account

import (
	"time"

	"gorm.io/gorm"
)

// User 用户账号实体
type User struct {
	ID          int64          `gorm:"primaryKey;autoIncrement"     json:"id"`
	UserName    string         `gorm:"size:64;not null"             json:"user_name"`
	Password    string         `gorm:"size:64;not null"             json:"-"` // bcrypt 哈希
	Phone       string         `gorm:"size:20;uniqueIndex;not null" json:"phone"`
	Email       *string        `gorm:"size:128;uniqueIndex"         json:"email,omitempty"`
	AvatarURL   string         `gorm:"size:512"                     json:"avatar_url,omitempty"`
	Version     int64          `gorm:"default:1"                    json:"-"` // 改密码/注销时 +1,令旧 JWT 失效
	CreatedAt   time.Time      `                                  json:"created_at"`
	LastLoginAt *time.Time     `                                  json:"last_login_at,omitempty"`
	DeletedAt   gorm.DeletedAt `                                  json:"-"`
}
