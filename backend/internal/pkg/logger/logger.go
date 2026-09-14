// Package logger 初始化全局结构化日志(slog),输出到本地文件。
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Init 把 slog 默认 logger 指向 path,同时输出一份到 stderr。
// 返回的 close 用于进程退出前关闭文件句柄。
//
// 不做日志轮转,文件会持续增长,生产环境需配合 logrotate。
func Init(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// O_APPEND:重启后追加,不覆盖已有日志
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}

	// 同时写 stderr,开发时终端能直接看到。
	// SetDefault 之后标准库 log 的 log.Printf 也会走这个 handler,
	// 所以 cmd/main.go 里现有的 log.Printf 无需改动。
	slog.SetDefault(slog.New(slog.NewJSONHandler(
		io.MultiWriter(os.Stderr, f),
		&slog.HandlerOptions{Level: slog.LevelInfo},
	)))

	return func() { _ = f.Close() }, nil
}
