package account

import (
	"time"

	"gorm.io/gorm"
)

// User 用户账号实体。
//
// 唯一约束与索引都不在 tag 里声明:本项目建表走 goose 迁移、不用 AutoMigrate,
// tag 里的声明不生效。phone / email 的 UNIQUE 和两个索引见
// migrations/001_create_users.sql
type User struct {
	ID        int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserName  string  `gorm:"size:64;not null"         json:"user_name"`
	Password  string  `gorm:"size:64;not null"         json:"-"` // bcrypt 哈希
	Phone     string  `gorm:"size:20;not null"         json:"phone"`
	Email     *string `gorm:"size:128"                 json:"email,omitempty"`
	AvatarURL string  `gorm:"size:512"                 json:"avatar_url,omitempty"`
	Version   int64   `gorm:"not null;default:1"       json:"-"` // 改密码/注销时 +1,令旧 JWT 失效
	// type 必须显式写:GORM 默认把 time.Time 映射成 timestamptz,而迁移里建的是 TIMESTAMP
	CreatedAt   time.Time      `gorm:"type:timestamp;not null" json:"created_at"`
	LastLoginAt *time.Time     `gorm:"type:timestamp"          json:"last_login_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"type:timestamp"          json:"-"`
}
