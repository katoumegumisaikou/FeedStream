package account

import (
	"time"

	"gorm.io/gorm"
)

// User 用户账号实体
type User struct {
	ID        int64          `gorm:"primaryKey;autoIncrement"`     // 主键 ID
	UserName  string         `gorm:"size:64;not null"`             // 用户名
	Password  string         `gorm:"size:255;not null"`            // 密码(bcrypt 哈希,严禁明文)
	Phone     string         `gorm:"size:20;uniqueIndex;not null"` // 手机号(唯一)
	Email     *string        `gorm:"size:128;uniqueIndex"`         // 邮箱(可空,唯一)
	AvatarURL string         `gorm:"size:512"`                     // 头像 URL(可空)
	CreatedAt time.Time      `gorm:"index"`                        // 注册时间
	UpdatedAt time.Time      `gorm:"index"`                        // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index"`                        // 软删除标记
}
