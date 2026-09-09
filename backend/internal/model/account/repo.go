package account

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// UserRepository 用户仓储接口(对外暴露的唯一契约)
type UserRepository interface {
	// Create 创建用户
	Create(ctx context.Context, user *User) error

	// FindByID 根据主键查询(自动过滤软删除)
	FindByID(ctx context.Context, id int64) (*User, error)

	// FindByPhone 根据手机号查询(自动过滤软删除)
	FindByPhone(ctx context.Context, phone string) (*User, error)

	// FindByUserName 根据用户名查询(自动过滤软删除)
	FindByUserName(ctx context.Context, userName string) (*User, error)

	// FindByEmail 根据邮箱查询(自动过滤软删除)
	FindByEmail(ctx context.Context, email string) (*User, error)

	// Update 全量更新(根据主键)
	Update(ctx context.Context, user *User) error

	// UpdateLastLoginAt 仅更新最近登录时间(避免全字段写)
	UpdateLastLoginAt(ctx context.Context, id int64, t time.Time) error

	// Delete 软删除(底层为 UPDATE deleted_at)
	Delete(ctx context.Context, id int64) error
}

// ErrNotFound 用户不存在
// 调用方应使用 errors.Is(err, account.ErrNotFound) 判断
var ErrNotFound = errors.New("account: 用户不存在")

// userRepository 基于 GORM 的实现(包内私有,外部只能通过接口访问)
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 构造 UserRepository
// 返回接口类型以隐藏实现细节,调用方无法访问 GORM 专属方法
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// 编译期断言:确保 userRepository 实现 UserRepository 接口
var _ UserRepository = (*userRepository)(nil)

// Create 实现 UserRepository.Create
func (r *userRepository) Create(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindByID 实现 UserRepository.FindByID
func (r *userRepository) FindByID(ctx context.Context, id int64) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByPhone 实现 UserRepository.FindByPhone
func (r *userRepository) FindByPhone(ctx context.Context, phone string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("phone = ?", phone).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUserName 实现 UserRepository.FindByUserName
func (r *userRepository) FindByUserName(ctx context.Context, userName string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("user_name = ?", userName).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail 实现 UserRepository.FindByEmail
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update 实现 UserRepository.Update(全量 Save)
func (r *userRepository) Update(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// UpdateLastLoginAt 实现 UserRepository.UpdateLastLoginAt
func (r *userRepository) UpdateLastLoginAt(ctx context.Context, id int64, t time.Time) error {
	return r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", id).
		Update("last_login_at", t).Error
}

// Delete 实现 UserRepository.Delete(软删除)
func (r *userRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&User{}, id).Error
}
