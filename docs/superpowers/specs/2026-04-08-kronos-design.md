# Kronos — 计划任务管理平台设计文档

> Date: 2026-04-08
> Status: Approved

## 1. 概述

Kronos 是一个独立的计划任务管理平台，融合三大能力：

1. **定时提醒（remind）** — 到点推送通知
2. **定时脚本（script）** — 定时执行 shell/Python 等脚本
3. **Agent 调用（agent）** — 通过 MCP 协议调用外部 AI Agent 执行复杂任务

项目以 Go 单体二进制形态交付，内置 CLI、RESTful API、MCP Server 和 Web UI，支持多用户和权限控制，为后续与 NGSOC 集成预留接口。

## 2. 技术选型

| 用途 | 技术 |
|------|------|
| 后端语言 | Go |
| CLI 框架 | cobra |
| HTTP 框架 | gin |
| ORM | gorm + sqlite driver + mysql driver |
| 调度引擎 | robfig/cron/v3 |
| 配置管理 | viper |
| JWT | golang-jwt/jwt/v5 |
| Redis 客户端 | go-redis/redis/v9（可选） |
| MCP SDK | mark3labs/mcp-go |
| 日志 | slog（标准库） |
| 前端框架 | Vue 3 + Vite 5 + Element Plus + Pinia + vue-router 4 |
| 前端嵌入 | go:embed |

## 3. 项目结构

```
kronos/
├── cmd/
│   ├── kronos/              # 入口
│   │   └── main.go
│   └── commands/            # CLI 子命令 (serve, task, user, ...)
├── internal/
│   ├── scheduler/           # 调度引擎核心
│   ├── executor/            # 任务执行器 (remind/script/agent)
│   ├── notifier/            # 通知模块 (feishu/webhook)
│   ├── mcp/                 # MCP Server (SSE)
│   ├── api/                 # RESTful API (路由+handler)
│   ├── auth/                # 认证鉴权 (JWT, 角色)
│   ├── model/               # 数据模型 (GORM)
│   └── store/               # 存储层 (SQLite + MySQL/Redis 可选)
├── web/                     # Vue 3 + Element Plus 前端
│   ├── src/
│   └── dist/                # 构建产物，go:embed 嵌入
├── configs/
│   └── kronos.yaml          # 配置文件模板
├── go.mod
└── Makefile
```

## 4. 配置文件

```yaml
server:
  host: 0.0.0.0
  port: 8360
  mode: release              # debug / release

auth:
  jwt_secret: ""
  access_token_ttl: 12h
  refresh_token_ttl: 7d

database:
  sqlite:
    path: ~/.kronos/kronos.db
  mysql:                      # 可选，NGSOC 联动
    enabled: false
    host: ""
    port: 3306
    user: ""
    password: ""
    database: ""
  redis:                      # 可选，缓存/队列
    enabled: false
    host: ""
    port: 6379
    password: ""
    db: 0

scheduler:
  max_concurrent: 10

mcp:
  enabled: true
  port: 8361

notifier:
  feishu:
    enabled: false
    webhook_url: ""
  webhook:
    enabled: false
    url: ""
    headers: {}
```

## 5. 数据模型

### users

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AUTO | 主键 |
| username | VARCHAR(64) UNIQUE | 用户名 |
| password_hash | VARCHAR(255) | 密码哈希 |
| display_name | VARCHAR(128) | 显示名 |
| role | ENUM('admin','user') | 角色 |
| status | ENUM('active','disabled') | 状态 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

### tasks

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AUTO | 主键 |
| name | VARCHAR(255) | 任务名 |
| type | ENUM('remind','script','agent') | 任务类型 |
| schedule_type | ENUM('cron','interval','once') | 调度类型 |
| schedule_expr | VARCHAR(255) | 调度表达式 |
| target | TEXT | 提醒文本/脚本路径/agent 指令 |
| args | JSON | 额外参数 |
| timeout | INT DEFAULT 300 | 执行超时(秒) |
| retry_count | INT DEFAULT 3 | 重试次数 |
| retry_interval | INT DEFAULT 5 | 重试间隔(秒) |
| notify_on | JSON | 通知条件 {"success": false, "fail": true} |
| notify_channel | VARCHAR(32) | 通知渠道 feishu/webhook |
| enabled | BOOLEAN DEFAULT true | 是否启用 |
| user_id | BIGINT FK | 归属用户 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

### task_runs

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AUTO | 主键 |
| task_id | BIGINT FK | 关联任务 |
| status | ENUM('running','success','failed','timeout','cancelled') | 执行状态 |
| started_at | DATETIME | 开始时间 |
| finished_at | DATETIME | 结束时间 |
| duration_ms | INT | 执行耗时(ms) |
| exit_code | INT | 退出码(script 类型) |
| output | TEXT | 执行输出(max 64KB) |
| error | TEXT | 错误信息 |
| retry_attempt | INT DEFAULT 0 | 当前重试次数 |

### notifications

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK AUTO | 主键 |
| task_run_id | BIGINT FK | 关联执行记录 |
| channel | VARCHAR(32) | 通知渠道 |
| status | ENUM('sent','failed') | 发送状态 |
| payload | TEXT | 发送内容 |
| response | TEXT | 渠道返回 |
| created_at | DATETIME | 创建时间 |

## 6. 核心模块设计

### 6.1 调度引擎 (internal/scheduler)

基于 `robfig/cron/v3` 封装：

- `Start()` — 从 DB 加载 enabled 任务，注册到调度器
- `Stop()` — 优雅停止，等待运行中任务完成
- `AddTask(task)` — 动态添加任务
- `RemoveTask(taskID)` — 动态移除
- `UpdateTask(task)` — 更新调度参数
- `RunNow(taskID)` — 手动立即触发
- `Status()` — 返回运行状态

并发控制：semaphore (`chan struct{}`) 限制 `max_concurrent`。

调度类型处理：
- **cron** — 直接注册 cron 表达式
- **interval** — 转换为 `@every` 语法
- **once** — 注册后执行一次，完成后自动 `enabled=false`

### 6.2 执行器 (internal/executor)

```go
type Executor interface {
    Execute(ctx context.Context, task *model.Task) *RunResult
}
```

三种实现：
- **RemindExecutor** — 直接触发通知，target 即提醒内容
- **ScriptExecutor** — `os/exec` 运行脚本，`context.WithTimeout` 超时控制，捕获 stdout/stderr，超 64KB 截断
- **AgentExecutor** — 通过 MCP Client 调用外部 Agent

失败后按 `retry_count` + `retry_interval` 重试。

### 6.3 通知模块 (internal/notifier)

```go
type Notifier interface {
    Send(ctx context.Context, msg *Notification) error
}
```

两种实现：
- **FeishuNotifier** — 飞书 Webhook（text / interactive card）
- **WebhookNotifier** — 通用 HTTP POST，自定义 headers

通知失败不阻塞主流程，记录到 notifications 表。

### 6.4 MCP Server (internal/mcp)

daemon 内置 SSE 端点（默认 port 8361），暴露以下 tools：

| Tool | 说明 |
|------|------|
| list_tasks | 列出任务（支持筛选） |
| create_task | 创建任务 |
| run_task | 立即执行指定任务 |
| get_task_status | 查询最近执行状态 |
| get_task_history | 查询执行历史 |
| enable_task | 启用任务 |
| disable_task | 禁用任务 |

Agent（Claude Code / Cursor 等）连接 `http://localhost:8361/mcp` 即可操作。

### 6.5 认证鉴权 (internal/auth)

- JWT（access 12h + refresh 7d）
- API 分层：公开接口（登录）+ 认证接口（其余）
- 角色：admin（管理所有任务和用户）/ user（仅管理自己的任务）
- CLI 本地操作可通过配置文件中的 admin token 跳过登录

## 7. API 设计

前缀 `/api/v1/`，统一响应 `{"code": 0, "message": "ok", "data": {...}}`

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /auth/login | 登录 |
| POST | /auth/refresh | 刷新 token |

### 用户管理（admin）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /users | 用户列表 |
| POST | /users | 创建用户 |
| PUT | /users/:id | 更新用户 |
| DELETE | /users/:id | 删除用户 |

### 任务管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /tasks | 任务列表（?type=&status=&page=&size=） |
| POST | /tasks | 创建任务 |
| GET | /tasks/:id | 任务详情 |
| PUT | /tasks/:id | 更新任务 |
| DELETE | /tasks/:id | 删除任务 |
| POST | /tasks/:id/run | 立即执行 |
| POST | /tasks/:id/enable | 启用 |
| POST | /tasks/:id/disable | 禁用 |

### 执行历史

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /tasks/:id/runs | 任务执行历史 |
| GET | /runs/:id | 单次执行详情 |
| GET | /runs/:id/output | 完整输出（流式） |

### 通知 & 仪表盘

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /notifications | 通知列表 |
| GET | /dashboard/stats | 统计概览 |
| GET | /dashboard/timeline | 最近执行时间线 |

## 8. CLI 命令

```bash
# 服务管理
kronos serve                      # 启动 daemon
kronos serve -d                   # 后台守护进程模式

# 任务管理
kronos task list                  # 列出所有任务
kronos task create                # 交互式创建
kronos task create -f task.yaml   # 从文件创建
kronos task info <id>             # 任务详情
kronos task run <id>              # 立即执行
kronos task enable <id>           # 启用
kronos task disable <id>          # 禁用
kronos task delete <id>           # 删除
kronos task logs <id>             # 查看执行日志

# 用户管理
kronos user list
kronos user create
kronos user delete <id>

# 系统
kronos status                     # 服务状态
kronos config init                # 生成默认配置
kronos version
```

## 9. Web UI

| 页面 | 功能 |
|------|------|
| 登录 | JWT 登录 |
| 仪表盘 | 统计卡片 + 执行时间线 + 成功率图表 |
| 任务列表 | 表格（筛选/搜索/启停/执行/删除） |
| 任务详情 | 基本信息 + cron 可视化 + 执行历史 |
| 创建/编辑任务 | 动态表单 + cron 辅助输入 |
| 执行详情 | 输出日志（ANSI 颜色）+ 耗时 + 状态 |
| 用户管理 | admin CRUD + 角色分配 |
| 通知记录 | 发送历史 + 状态筛选 |

UI 风格：火山引擎风格（primary #165dff，白底卡片，12px 圆角，1px #e5e6eb 边框）。

## 10. 启动流程

```
kronos serve
├── 1. 加载配置 (kronos.yaml / 环境变量 / CLI flags)
├── 2. 初始化存储
│     ├── SQLite 连接 + auto migrate
│     ├── MySQL 连接（如 enabled）
│     └── Redis 连接（如 enabled）
├── 3. 检查默认 admin 用户，不存在则创建
├── 4. 启动调度引擎（从 DB 加载 enabled 任务）
├── 5. 启动 API Server (port 8360)
├── 6. 启动 MCP Server (port 8361, 如 enabled)
├── 7. 注册信号处理 (SIGINT/SIGTERM)
└── 8. 就绪日志
```

优雅关停：停止接收新请求 → 等待运行中任务完成（最多 30s）→ 关闭 DB 连接 → 退出。

## 11. 错误处理 & 容错

| 场景 | 处理 |
|------|------|
| 脚本超时 | context.WithTimeout 取消，标记 timeout |
| 脚本失败 | 按 retry_count 重试，最终通知 |
| 通知失败 | 记录 notifications 表，不阻塞 |
| daemon 崩溃重启 | 从 DB 恢复任务，running 状态标记为 failed |
| SQLite 写冲突 | WAL 模式 + busy_timeout |
| MySQL/Redis 断连 | 自动重连，不影响核心调度 |

## 12. 部署

**开发环境：**
```bash
go run cmd/kronos/main.go serve          # 后端
cd web && npm run dev                     # 前端（Vite proxy → :8360）
```

**生产环境：**
```bash
make build                                # 前端打包 → go:embed → 单二进制
./bin/kronos serve -d --config /etc/kronos/kronos.yaml
```

支持 systemd 管理，单二进制无外部依赖。
