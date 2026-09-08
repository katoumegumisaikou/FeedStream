package database

import (
	"errors"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"

	"feed-system/internal/database/migrations"
)
// Migrator 封装 goose,使用 embed.FS 内嵌的 SQL 迁移文件
type Migrator struct {
	db *gorm.DB
	fs migrations.FS
}

// NewMigrator 创建迁移器,使用 embed.FS 加载 migrations 目录下的所有 SQL
func NewMigrator(db *gorm.DB) (*Migrator, error) {
	if db == nil {
		return nil, errors.New("db 不能为空")
	}
	return &Migrator{
		db: db,
		fs: migrations.Files,
	}, nil
}

// Up 执行所有未应用的迁移
func (m *Migrator) Up() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("获取 *sql.DB 失败: %w", err)
	}

	goose.SetBaseFS(m.fs)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("设置方言失败: %w", err)
	}

	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("执行迁移失败: %w", err)
	}
	return nil
}

// Down 回滚最后一次迁移
func (m *Migrator) Down() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("获取 *sql.DB 失败: %w", err)
	}

	goose.SetBaseFS(m.fs)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("设置方言失败: %w", err)
	}

	if err := goose.Down(sqlDB, "."); err != nil {
		return fmt.Errorf("回滚迁移失败: %w", err)
	}
	return nil
}

// Status 打印当前迁移状态(用于运维排障)
func (m *Migrator) Status() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("获取 *sql.DB 失败: %w", err)
	}

	goose.SetBaseFS(m.fs)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("设置方言失败: %w", err)
	}

	return goose.Status(sqlDB, ".")
}
