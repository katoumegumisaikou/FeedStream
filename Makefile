# ============================================================
# feed-system 项目顶层 Makefile
# 用法:
#   make help           列出所有可用 target
#   make dev            并发启动后端 + 前端开发服务器
#   make build          全量构建(后端二进制 + 前端静态产物)
#   make clean          清理所有构建产物
# ============================================================

# ============ 路径变量 ============
BACKEND_DIR  := backend
FRONTEND_DIR := frontend
BACKEND_BIN  := $(BACKEND_DIR)/bin/server

# ============ 默认目标 ============
.DEFAULT_GOAL := help

# ============ help(自动扫描 ## 注释) ============
.PHONY: help
help: ## 显示所有可用 target
	@echo "可用 target:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "示例:make dev(开发) / make build(生产构建) / make clean"

# ============================================================
# 整合
# ============================================================
.PHONY: all
all: build ## 等价于 make build

.PHONY: build
build: backend-build frontend-build ## 全量构建(后端二进制 + 前端静态产物)

.PHONY: dev
dev: ## 并发启动后端(8080)+ 前端 dev server(5173)
	@echo "▶ 启动后端 + 前端(各自独立进程,Ctrl+C 全部退出)..."
	@trap 'kill 0' EXIT; \
	( cd $(BACKEND_DIR)  && go run ./cmd/main.go ) & \
	( cd $(FRONTEND_DIR) && npm run dev )

.PHONY: clean
clean: ## 清理所有构建产物
	rm -rf $(BACKEND_DIR)/bin $(FRONTEND_DIR)/dist

# ============================================================
# 后端 (Go)
# ============================================================
.PHONY: backend-build
backend-build: ## 编译后端二进制到 backend/bin/server
	cd $(BACKEND_DIR) && go build -o bin/server ./cmd

.PHONY: backend-run
backend-run: ## 直接 go run 启动后端(自动跑数据库迁移)
	cd $(BACKEND_DIR) && go run ./cmd/main.go

.PHONY: backend-test
backend-test: ## 跑后端单元测试
	cd $(BACKEND_DIR) && go test ./...

.PHONY: backend-lint
backend-lint: ## go vet 检查
	cd $(BACKEND_DIR) && go vet ./...

.PHONY: backend-tidy
backend-tidy: ## 整理 go.mod / go.sum
	cd $(BACKEND_DIR) && go mod tidy

.PHONY: backend-fmt
backend-fmt: ## gofmt 格式化所有 Go 源码
	cd $(BACKEND_DIR) && gofmt -w .

# ============================================================
# 前端 (React + TS + Vite)
# ============================================================
.PHONY: frontend-install
frontend-install: ## 安装前端依赖
	cd $(FRONTEND_DIR) && npm install

.PHONY: frontend-dev
frontend-dev: ## 启动前端 dev server(5173,需后端在 8080)
	cd $(FRONTEND_DIR) && npm run dev

.PHONY: frontend-build
frontend-build: ## 前端生产构建(产物在 frontend/dist/)
	cd $(FRONTEND_DIR) && npm run build

.PHONY: frontend-typecheck
frontend-typecheck: ## 前端 TypeScript 类型检查
	cd $(FRONTEND_DIR) && npm run typecheck

.PHONY: frontend-preview
frontend-preview: ## 预览前端生产构建
	cd $(FRONTEND_DIR) && npm run preview

# ============================================================
# 强制所有 target 走 .PHONY(防止跟同名文件冲突)
# ============================================================
.PHONY: help all build dev clean \
        backend-build backend-run backend-test backend-lint backend-tidy backend-fmt \
        frontend-install frontend-dev frontend-build frontend-typecheck frontend-preview