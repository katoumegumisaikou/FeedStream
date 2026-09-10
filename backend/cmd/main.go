// feed_system 后端入口
// 启动流程:加载配置 → 连接数据库 → 执行迁移 → 连接 Redis → 装配 account 模块 → 启动 HTTP server
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"feed-system/internal/database"
	"feed-system/internal/model/account"
)

// getEnv 读环境变量,带默认值
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvBool 读 bool 环境变量(支持 1/true/yes/TRUE 等大小写),其他值走 fallback
func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "TRUE", "True", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "False", "no", "NO", "off", "OFF":
		return false
	}
	return fallback
}

// main 启动入口
func main() {
	// 1. 加载数据库配置
	cfg := database.LoadConfigFromEnv()

	// 2. 建立数据库连接
	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	log.Printf("✓ 已连接到 PostgreSQL: %s:%s/%s", cfg.Host, cfg.Port, cfg.DBName)

	// 3. 执行迁移
	migrator, err := database.NewMigrator(db)
	if err != nil {
		log.Fatalf("创建迁移器失败: %v", err)
	}
	if err := migrator.Up(); err != nil {
		log.Fatalf("执行迁移失败: %v", err)
	}
	log.Println("✓ 数据库迁移完成")

	// 4. 连接 Redis(失败不致命,降级为 nil → SMS / 登录锁定不可用,但鉴权仍走 DB)
	rdb := mustRedisClient()

	// 5. 装配 account 模块
	userRepo := account.NewUserRepository(db)
	devMode := getEnvBool("SMS_DEV_MODE", true) // 默认 dev 模式:短信验证码会回显给前端
	accountSvc := account.NewAccountService(userRepo, rdb, devMode)
	accountHandler := account.NewAccountHandler(accountSvc)

	// 6. 创建 gin engine
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 7. 注册 account 路由(/api/v1/...)
	v1 := r.Group("/api/v1")
	account.RegisterRouter(v1, accountHandler, db, rdb)

	// 8. 启动 HTTP server
	addr := getEnv("HTTP_ADDR", ":8080")
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	go func() {
		log.Printf("✓ HTTP server 监听 %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server 启动失败: %v", err)
		}
	}()

	// 9. 优雅退出:收到信号后给 5 秒做收尾
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("收到信号 %s,准备退出...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server 关闭失败: %v", err)
	}
	if rdb != nil {
		_ = rdb.Close()
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
	log.Println("bye")
}

// mustRedisClient 创建 Redis 客户端,连接失败只 warn 不 fatal(rdb 返回 nil)
// SMS / 登录锁定等依赖 Redis 的功能在 rdb=nil 时会跳过,鉴权中间件也会回源 DB
func mustRedisClient() *redis.Client {
	addr := getEnv("REDIS_HOST", "localhost") + ":" + getEnv("REDIS_PORT", "6379")
	password := getEnv("REDIS_PASSWORD", "")

	rdb := redis.NewClient(&redis.Options{
		Addr:        addr,
		Password:    password,
		DB:          0,
		DialTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("⚠ Redis 连接失败(%s),降级为 nil:SMS / 登录锁定不可用,鉴权走 DB。错误:%v", addr, err)
		_ = rdb.Close()
		return nil
	}
	log.Printf("✓ 已连接到 Redis: %s", addr)
	return rdb
}