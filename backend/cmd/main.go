// feed_system 后端入口
// 启动流程:加载配置 → 连接数据库 → 执行迁移 → 等待退出信号
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"feed-system/internal/database"
)

func main() {
	// 1. 加载数据库配置
	cfg := database.LoadConfigFromEnv()

	// 2. 建立数据库连接
	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	log.Printf("已连接到 PostgreSQL: %s:%s/%s", cfg.Host, cfg.Port, cfg.DBName)

	// 3. 执行迁移
	migrator, err := database.NewMigrator(db)
	if err != nil {
		log.Fatalf("创建迁移器失败: %v", err)
	}
	if err := migrator.Up(); err != nil {
		log.Fatalf("执行迁移失败: %v", err)
	}
	log.Println("数据库迁移完成")

	// 4. 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("收到信号 %s,准备退出...", sig)

	// 5. 关闭数据库连接
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
	log.Println("bye")
}
