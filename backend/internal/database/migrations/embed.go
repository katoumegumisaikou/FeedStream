// Package migrations 集中管理 SQL 迁移文件。
// 用 embed.FS 内嵌进二进制,部署时无需随包携带 migrations 目录。
package migrations

import "embed"

type FS = embed.FS

//go:embed *.sql
var Files embed.FS
