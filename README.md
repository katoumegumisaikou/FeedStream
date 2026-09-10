# feed_system

Go + React 全栈视频 feed 系统。**当前阶段:账号(account)模块已实现,其它业务模块待开发。**

## 技术栈

| 层 | 选型 |
|----|------|
| 后端 | Go 1.26 + Gin + GORM + PostgreSQL + Redis + JWT |
| 前端 | React 18 + TypeScript + Vite + Ant Design + Axios |
| 迁移 | Goose + embed.FS |

## 目录结构

```
feed-system/
├── backend/                 Go 后端(Gin + GORM + Goose)
│   ├── cmd/main.go          入口:启动 DB / Redis / HTTP server
│   ├── internal/
│   │   ├── model/account/   账号模块(handler / service / repo / auth middleware / router)
│   │   ├── pkg/             通用工具(token / errs / response)
│   │   ├── util/            校验工具(password / username)
│   │   └── database/        DB 连接 + Goose 迁移
│   └── .env.example         后端环境变量样例
├── frontend/                React 前端(Vite + Ant Design)
│   ├── src/
│   │   ├── api/             axios 实例 + auth API
│   │   ├── pages/           Login / SmsLogin / Register / ForgotPassword
│   │   ├── components/      SmsCodeButton(60s 倒计时)
│   │   ├── layouts/         AuthLayout(卡片居中布局)
│   │   └── types/           与后端 DTO 对齐的 TS 类型
│   └── vite.config.ts       /api 代理到 :8080
├── docker-compose.yml       PostgreSQL + Redis 一键起
└── Makefile                 聚合所有常用操作
```

## 快速开始(本地开发)

### 前置条件

- Go 1.26+
- Node 20+
- Docker + Docker Compose

### 步骤

```bash
# 1. 启动 PostgreSQL + Redis(后台)
docker compose up -d

# 2. (可选)改后端环境变量默认值
cp backend/.env.example backend/.env
$EDITOR backend/.env

# 3. 一键启动后端 + 前端(并发,Ctrl+C 全部退出)
make dev

# 或分开启动:
make backend-run       # 后端 :8080
make frontend-dev      # 前端 :5173
```

打开浏览器访问 `http://localhost:5173`,看到登录页即成功。

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` / `DB_SSLMODE` | `localhost:5432` / `postgres` / `postgres` / `feed_system` / `disable` | PostgreSQL |
| `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASSWORD` | `localhost:6379` / 空 | Redis(可降级为 nil) |
| `HTTP_ADDR` | `:8080` | HTTP 监听地址 |
| `SMS_DEV_MODE` | `true` | true 时验证码回显到前端,生产务必 false |
| `VITE_API_TARGET`(前端) | `http://localhost:8080` | vite proxy 目标 |

## 联调测试 account 模块

启动后,可以这样测一遍:

1. 打开 `http://localhost:5173/register`
2. 填一个手机号 + 点"获取验证码"(dev 模式响应里有 `sms_code`)
3. 把验证码填进表单 + 设密码 + 提交
4. 注册成功后应跳转到首页(`/`,目前是 placeholder)
5. 登出 → 用 `/login`(密码登录)再登录
6. 改密 → `/forgot-password`,收验证码,改完应自动登录

后端日志会打印关键操作:`✓ 已连接到 PostgreSQL`、`✓ 已连接到 Redis`、`✓ HTTP server 监听 :8080` 等。

## 常用命令

```bash
make help               # 列出所有 target
make build              # 全量构建(后端二进制 + 前端 dist)
make dev                # 并发启动后端 + 前端
make backend-test       # 跑后端单元测试
make backend-lint       # go vet
make frontend-typecheck # tsc --noEmit
make clean              # 清理构建产物
```

完整列表见 [Makefile](Makefile)。

## 鉴权机制

- token 通过 **HttpOnly cookie** 下发(防 XSS 窃取)
- access_token 2h,refresh_token 30d
- 改密 / 注销会递增 `users.version`,旧 token 自动失效(中间件从 Redis 缓存读 version,Redis miss 时回源 DB)

## 当前状态

- ✅ 后端基础设施:Go module + DB / Redis 连接 + Goose + 错误体系 + JWT + HttpOnly cookie
- ✅ account 模块:Register / Login(密码) / Login(短信) / Logout / Refresh / ChangePassword / SendSmsCode / GetProfile / UpdateProfile
- ✅ 前端 4 个页面 + SMS 倒计时按钮 + axios 拦截器
- 🚧 其它业务模块(feed / video / user profile 等)待开发