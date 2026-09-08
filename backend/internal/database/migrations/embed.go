// Package migrations 集中管理数据库迁移文件(SQL)。
// 使用 embed.FS 将 SQL 内嵌到二进制中,部署时无需携带 migrations 目录。
package migrations

import "embed"

// FS 是 migrations 目录的内嵌文件系统别名
type FS = embed.FS

//go:embed *.sql
var Files embed.FS
