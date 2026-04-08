# Kronos Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build Kronos, a scheduled task management platform with Go backend, CLI, MCP Server, and Vue 3 Web UI.

**Architecture:** Go single-binary monolith. Backend provides RESTful API (gin), scheduling engine (robfig/cron), task execution (remind/script/agent), notifications (feishu/webhook), and MCP Server (SSE). Frontend is Vue 3 + Element Plus SPA embedded via go:embed. SQLite primary storage with optional MySQL/Redis connections.

**Tech Stack:** Go 1.22+, cobra, gin, gorm, robfig/cron/v3, viper, golang-jwt/jwt/v5, mark3labs/mcp-go, slog | Vue 3, Vite 5, Element Plus, Pinia, vue-router 4

**Spec:** `docs/superpowers/specs/2026-04-08-kronos-design.md`

---

## File Structure

```
kronos/
├── cmd/kronos/main.go                    # Entry point, root cobra command
├── cmd/commands/
│   ├── serve.go                          # serve command (daemon startup)
│   ├── task.go                           # task subcommands (list/create/info/run/enable/disable/delete/logs)
│   ├── user.go                           # user subcommands (list/create/delete)
│   ├── status.go                         # status command
│   ├── config.go                         # config init command
│   └── version.go                        # version command
├── internal/
│   ├── config/config.go                  # Viper config loading, auto-generate secrets
│   ├── model/
│   │   ├── user.go                       # User GORM model
│   │   ├── task.go                       # Task GORM model
│   │   ├── task_run.go                   # TaskRun GORM model
│   │   └── notification.go              # Notification GORM model
│   ├── store/
│   │   ├── database.go                   # DB init (SQLite + optional MySQL/Redis), auto-migrate, file lock
│   │   ├── user_store.go                 # User CRUD operations
│   │   ├── task_store.go                 # Task CRUD + query operations
│   │   ├── task_run_store.go             # TaskRun CRUD + query operations
│   │   └── notification_store.go         # Notification CRUD + query operations
│   ├── auth/
│   │   ├── jwt.go                        # JWT token generation/validation
│   │   └── middleware.go                 # Gin auth middleware + role check
│   ├── scheduler/scheduler.go            # Scheduling engine (cron/interval/once)
│   ├── executor/
│   │   ├── executor.go                   # Executor interface + dispatch + retry logic
│   │   ├── remind.go                     # RemindExecutor
│   │   ├── script.go                     # ScriptExecutor
│   │   └── agent.go                      # AgentExecutor (MCP client, placeholder)
│   ├── notifier/
│   │   ├── notifier.go                   # Notifier interface + dispatch
│   │   ├── feishu.go                     # Feishu webhook notifier
│   │   └── webhook.go                    # Generic webhook notifier
│   ├── api/
│   │   ├── router.go                     # Gin router setup, middleware, static files
│   │   ├── response.go                   # Unified response helpers
│   │   ├── auth_handler.go               # Login, refresh endpoints
│   │   ├── user_handler.go               # User CRUD endpoints
│   │   ├── task_handler.go               # Task CRUD + run/enable/disable endpoints
│   │   ├── run_handler.go                # TaskRun query + output endpoints
│   │   ├── notification_handler.go       # Notification list endpoint
│   │   └── dashboard_handler.go          # Stats + timeline endpoints
│   ├── mcp/server.go                     # MCP SSE server with tools
│   └── server/server.go                  # Top-level server orchestration (startup/shutdown)
├── web/                                  # Vue 3 SPA (separate task)
├── configs/kronos.yaml                   # Config template
├── Makefile                              # Build targets
├── go.mod
└── .gitignore
```

---

## Phase 1: Backend Core

### Task 1: Project Skeleton + Config

**Files:**
- Create: `cmd/kronos/main.go`
- Create: `cmd/commands/serve.go`
- Create: `cmd/commands/version.go`
- Create: `internal/config/config.go`
- Create: `configs/kronos.yaml`
- Create: `go.mod`
- Create: `Makefile`
- Create: `.gitignore`

**What to build:**
Initialize Go module (`github.com/pstrr/kronos` or similar). Set up cobra root command in main.go with `serve` and `version` subcommands. Implement config loading via viper: read from `kronos.yaml` (default `~/.kronos/kronos.yaml`), support `--config` flag override, environment variable binding (`KRONOS_` prefix). Config struct should match spec Section 4 exactly. Auto-generate `jwt_secret` and `mcp.token` if empty on first load and write back to config file. Create Makefile with `build`, `dev`, `clean` targets. The `serve` command should just load config and print it for now.

**Key details:**
- `~/.kronos/` directory auto-created if missing
- Config file path resolution: `--config` flag > `./kronos.yaml` > `~/.kronos/kronos.yaml`
- slog setup based on `log.level` and `log.format` config
- `make build` should: `cd web && npm run build` then `go build -o bin/kronos ./cmd/kronos`

**Verify:** `go run cmd/kronos/main.go version` prints version; `go run cmd/kronos/main.go serve` loads config and prints it

**Commit:** `feat: project skeleton with config and CLI framework`

---

### Task 2: Data Models + Store Layer

**Files:**
- Create: `internal/model/user.go`
- Create: `internal/model/task.go`
- Create: `internal/model/task_run.go`
- Create: `internal/model/notification.go`
- Create: `internal/store/database.go`
- Create: `internal/store/user_store.go`
- Create: `internal/store/task_store.go`
- Create: `internal/store/task_run_store.go`
- Create: `internal/store/notification_store.go`

**What to build:**
GORM models matching spec Section 5 exactly. All four tables with proper field types, JSON fields using `datatypes.JSON`, soft delete on tasks via `gorm.DeletedAt`. Store layer: `database.go` handles SQLite connection with WAL mode + busy_timeout, auto-migrate all models, optional MySQL/Redis connections (just connect and store the client, no usage yet). Each `*_store.go` provides CRUD + paginated list queries. File lock (`flock`) on SQLite DB path to prevent multi-instance. Startup cleanup: mark any `running` task_runs as `failed`.

**Key details:**
- SQLite: `?_journal_mode=WAL&_busy_timeout=5000`
- User password hashing: bcrypt
- Task `args` and `notify_on` fields: use `datatypes.JSON` from gorm
- Paginated list helper: accept page/size, return items + total count
- TaskStore needs: `ListByUser(userID, filters)`, `ListEnabled()` for scheduler
- TaskRunStore needs: `ListByTask(taskID, page, size)`, `GetLatestByTask(taskID)`
- Indexes via GORM tags as specified in the spec

**Verify:** `go build ./...` compiles; write a quick test in `internal/store/database_test.go` that opens SQLite, migrates, creates a user and task

**Commit:** `feat: GORM models and store layer with SQLite`

---

### Task 3: Auth (JWT + Middleware)

**Files:**
- Create: `internal/auth/jwt.go`
- Create: `internal/auth/middleware.go`

**What to build:**
JWT module: `GenerateAccessToken(user)`, `GenerateRefreshToken(user)`, `ParseToken(tokenString)` using `golang-jwt/jwt/v5`. Claims include `user_id`, `username`, `role`, `exp`. Gin middleware: `AuthRequired()` extracts and validates JWT from `Authorization: Bearer <token>` header, sets user info in gin context. `AdminRequired()` middleware chains AuthRequired + role check. Helper `GetCurrentUser(c *gin.Context)` to extract user from context.

**Key details:**
- TTL values from config (`access_token_ttl`, `refresh_token_ttl`)
- Token parsing errors should return 401 with clear message
- Disabled users (`status != active`) should be rejected even with valid token

**Verify:** Unit test: generate token → parse token → verify claims; generate expired token → parse fails

**Commit:** `feat: JWT auth with gin middleware`

---

### Task 4: Notifier Module

**Files:**
- Create: `internal/notifier/notifier.go`
- Create: `internal/notifier/feishu.go`
- Create: `internal/notifier/webhook.go`

**What to build:**
`Notifier` interface with `Send(ctx, *Notification) error`. `Manager` struct that holds configured notifiers and dispatches by channel name. `FeishuNotifier`: POST to webhook URL with message card format (title + content + status + task name + timestamp). `WebhookNotifier`: POST to URL with custom headers and JSON body. Manager also records notification results to `notification_store`.

**Key details:**
- Feishu message format: interactive card with title, status color (green=success, red=fail), task name, output summary (truncated to 500 chars)
- Webhook body: `{"task_name": "", "status": "", "output": "", "error": "", "timestamp": ""}`
- 10 second HTTP timeout per notification attempt
- All sends are non-blocking (fire in goroutine), errors logged + stored but don't affect caller

**Verify:** Unit test with mock HTTP server: send feishu notification → verify request format; send webhook → verify headers/body

**Commit:** `feat: notification module with feishu and webhook support`

---

### Task 5: Executor Module

**Files:**
- Create: `internal/executor/executor.go`
- Create: `internal/executor/remind.go`
- Create: `internal/executor/script.go`
- Create: `internal/executor/agent.go`

**What to build:**
`Executor` interface: `Execute(ctx, *model.Task) *RunResult`. `RunResult` struct: `Status`, `ExitCode`, `Output`, `Error`, `Duration`. Dispatch function that routes by task type. `RemindExecutor`: calls notifier directly with task target as message content, always returns success. `ScriptExecutor`: runs `target` via `os/exec.CommandContext`, captures combined stdout+stderr, truncates at 64KB, respects context timeout. `AgentExecutor`: placeholder that returns "agent execution not yet implemented". Top-level `Run(task)` function that: creates task_run record → executes → updates task_run with result → handles retry on failure → triggers notification per `notify_on` config → updates task's `last_run_at`/`last_status`.

**Key details:**
- ScriptExecutor: if target ends with `.py`, run via `python3`; if `.sh` or no extension, run via `sh -c`; otherwise execute directly
- Output truncation: keep last 64KB if output exceeds limit
- Retry loop: sleep `retry_interval` between attempts, create new task_run per attempt with incremented `retry_attempt`
- Run function needs access to store and notifier (dependency injection via struct)

**Verify:** Test ScriptExecutor with `echo hello` → success + output; test with `sleep 10` and 1s timeout → timeout status; test with `exit 1` → failed status

**Commit:** `feat: task executors with remind, script, and agent support`

---

### Task 6: Scheduler Engine

**Files:**
- Create: `internal/scheduler/scheduler.go`

**What to build:**
`Scheduler` struct wrapping `robfig/cron/v3` instance. On `Start()`: load all enabled tasks from store, register each with the cron library. Maintains a map of `taskID → cron.EntryID` for dynamic add/remove/update. Semaphore channel (`chan struct{}` of size `max_concurrent`) to limit parallel execution. Each task execution: acquire semaphore → call executor.Run() in goroutine → release semaphore. `RunNow(taskID)`: bypass schedule, execute immediately (still respects semaphore). `UpdateTask`: remove old entry + add new one. On `Stop()`: call `cron.Stop()`, wait for running goroutines via WaitGroup (max 30s).

**Key details:**
- cron type: register expression directly with `cron.New(cron.WithSeconds())` for second-level granularity
- interval type: convert to `@every <duration>` syntax
- once type: use `time.AfterFunc(duration)` where duration = target time - now. If target is in the past and task never ran (`last_status` is empty), execute immediately. If already ran, skip.
- After each execution, compute and update `next_run_at` on the task record
- `triggered_by` should be "scheduler" for automatic runs

**Verify:** Create a task with `@every 2s` schedule, start scheduler, verify it fires 2-3 times within 6 seconds, then stop

**Commit:** `feat: scheduling engine with cron, interval, and once support`

---

### Task 7: REST API

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/response.go`
- Create: `internal/api/auth_handler.go`
- Create: `internal/api/user_handler.go`
- Create: `internal/api/task_handler.go`
- Create: `internal/api/run_handler.go`
- Create: `internal/api/notification_handler.go`
- Create: `internal/api/dashboard_handler.go`

**What to build:**
Gin router with all endpoints from spec Section 7. `response.go`: `Success(c, data)`, `Error(c, code, message)`, `PagedSuccess(c, items, total, page, size)` helpers. `auth_handler`: login (validate username/password → return tokens), refresh (validate refresh token → return new access token). `user_handler`: admin-only CRUD. `task_handler`: CRUD with user scoping (admin sees all, user sees own), run/enable/disable actions. Scheduler integration: when task is created/updated/deleted/enabled/disabled, call corresponding scheduler method. `run_handler`: list runs by task, get single run detail, stream output endpoint. `dashboard_handler`: stats (total tasks, today's runs, success rate, active tasks) + timeline (last 20 runs with task name).

**Key details:**
- All list endpoints use pagination helper from response.go
- Task creation/update: validate schedule_expr (try parsing cron/interval/ISO8601)
- `POST /tasks/:id/run`: set `triggered_by` to `"api:<user_id>"`
- `GET /runs/:id/output`: return full output as plain text with `Content-Type: text/plain`
- Dashboard stats query: use raw SQL or GORM aggregation for efficiency
- Router setup: API routes under `/api/v1/`, static file serving for web UI under `/` (go:embed placeholder for now)

**Verify:** `go run cmd/kronos/main.go serve` starts API server; test login → create task → list tasks → run task → get runs via curl

**Commit:** `feat: REST API with all endpoints`

---

### Task 8: Server Orchestration + Serve Command

**Files:**
- Create: `internal/server/server.go`
- Modify: `cmd/commands/serve.go`

**What to build:**
`server.go`: `Server` struct that owns all components (config, store, scheduler, executor, notifier, api router). `Start()` method implements the full startup sequence from spec Section 10: load config → init store (SQLite + optional MySQL/Redis) → create default admin → acquire file lock → cleanup stale runs → start scheduler → start API server → register signal handler → block until signal. `Shutdown()` method: stop accepting requests → stop scheduler (wait 30s) → close DB → release lock. Integrate into `serve` command: create Server and call Start().

**Key details:**
- Default admin: username `admin`, password `admin` (log a warning suggesting password change)
- `-d` flag for daemon mode: use `github.com/sevlyar/go-daemon` or simply document "use systemd"—recommend keeping it simple and just supporting foreground mode + systemd for now, skip `-d` to avoid complexity
- Signal handling: SIGINT and SIGTERM trigger graceful shutdown
- All components wired via dependency injection (pass store/notifier/scheduler references)

**Verify:** `go run cmd/kronos/main.go serve` → full startup sequence → Ctrl+C triggers graceful shutdown; verify API is accessible during run

**Commit:** `feat: server orchestration with full startup/shutdown lifecycle`

---

## Phase 2: CLI Commands

### Task 9: CLI Task + User + Status Commands

**Files:**
- Create: `cmd/commands/task.go`
- Create: `cmd/commands/user.go`
- Create: `cmd/commands/status.go`
- Create: `cmd/commands/config.go`

**What to build:**
All CLI subcommands from spec Section 8. CLI commands work by making HTTP requests to the running daemon's API (default `http://localhost:8360`). Config for CLI: read server address from config file or `--server` flag. Auth: `kronos task list` etc. need a token—store it in `~/.kronos/cli-token` after `kronos user create` generates it, or read from config's `auth.jwt_secret` to generate an admin token directly for local CLI usage.

`task` subcommands: list (table output), create (interactive prompts or `-f` YAML file), info (detailed view), run, enable, disable, delete, logs (last 10 runs with output). `user` subcommands: list, create (interactive), delete. `status`: GET to a `/api/v1/status` endpoint (add this endpoint) showing uptime, task counts, running tasks. `config init`: write default `kronos.yaml` to `~/.kronos/kronos.yaml`.

**Key details:**
- Table output: use `text/tabwriter` for aligned columns
- Interactive create: prompt for name, type, schedule_type, schedule_expr, target, timeout etc.
- `task create -f task.yaml`: parse YAML with same structure as the task JSON API
- `task logs <id>`: show last 10 runs in reverse chronological order, with status/duration/output preview
- Add `GET /api/v1/status` endpoint to api router (server uptime, scheduler stats)

**Verify:** Start server in one terminal; in another: `kronos task create -f` with a test YAML → `kronos task list` → `kronos task run <id>` → `kronos task logs <id>`

**Commit:** `feat: CLI commands for task, user, status, and config management`

---

## Phase 3: MCP Server

### Task 10: MCP Server

**Files:**
- Create: `internal/mcp/server.go`
- Modify: `internal/server/server.go` (wire MCP into startup)

**What to build:**
MCP SSE server using `mark3labs/mcp-go`. Bind to `mcp.host:mcp.port` from config. Implement 7 tools from spec Section 6.4: `list_tasks`, `create_task`, `run_task`, `get_task_status`, `get_task_history`, `enable_task`, `disable_task`. Each tool maps to the corresponding store/scheduler operation. Bearer token auth: validate `Authorization` header against `mcp.token` from config. Wire into server.go startup/shutdown.

**Key details:**
- `run_task`: set `triggered_by` to `"mcp"`
- `list_tasks`: accept optional `type` and `enabled` filter params
- `get_task_history`: accept `task_id` and optional `limit` (default 10)
- `get_task_status`: return task info + latest run info combined
- All tool responses as JSON strings matching the API response format
- If `mcp.enabled` is false, skip startup entirely

**Verify:** Start server with MCP enabled; use `curl` to test SSE endpoint; or configure Claude Code's `.mcp.json` to connect and test `list_tasks`

**Commit:** `feat: MCP Server with task management tools`

---

## Phase 4: Web UI

### Task 11: Vue 3 Project Scaffold + Layout

**Files:**
- Create: `web/` (Vite + Vue 3 + Element Plus + Pinia + vue-router project)
- Key files: `web/src/main.js`, `web/src/App.vue`, `web/src/router/index.js`, `web/src/stores/auth.js`, `web/src/utils/request.js`, `web/src/layout/MainLayout.vue`, `web/src/views/Login.vue`
- Modify: `internal/api/router.go` (add go:embed static serving)

**What to build:**
Scaffold Vue 3 project with Vite. Install Element Plus, Pinia, vue-router, axios. Set up `request.js` (axios instance with baseURL `/api/v1/`, JWT interceptor from Pinia auth store, response unwrapper, error handling with ElMessage). Auth store: login/logout/refresh, token persistence in localStorage. Router with auth guard (redirect to login if no token). `MainLayout.vue`: sidebar (logo + nav menu) + header (user info + logout) + main content area. Login page with username/password form. Vite proxy config: `/api` → `http://localhost:8360`. Update `internal/api/router.go` to serve `web/dist/` via `go:embed` for production, with SPA fallback (all non-API routes serve index.html).

**Key details:**
- UI style: fire mountain engine style — primary #165dff, text #1d2129, secondary #86909c, bg #f2f3f5, sidebar white, 36px menu items, 4px active radius, active #e8f3ff, cards 12px radius + 1px #e5e6eb border
- Sidebar menu items: Dashboard, Tasks, Users (admin only), Notifications
- go:embed: use `//go:embed all:web/dist` with a placeholder `web/dist/.gitkeep` so it compiles without building frontend first

**Verify:** `cd web && npm run dev` → login page renders → login with admin/admin → redirects to dashboard layout

**Commit:** `feat: Vue 3 scaffold with auth, layout, and login`

---

### Task 12: Dashboard Page

**Files:**
- Create: `web/src/views/Dashboard.vue`
- Create: `web/src/api/dashboard.js`

**What to build:**
Dashboard with: 4 stat cards at top (total tasks, today's runs, success rate %, active tasks), recent execution timeline below (table showing last 20 runs: task name, status tag, duration, triggered_by, time). Stat cards use Element Plus `el-statistic` or custom card components. Status shown as colored tags (success=green, failed=red, timeout=orange, running=blue). Auto-refresh every 30 seconds.

**Key details:**
- Success rate: `success_count / total_count * 100` from stats API
- Timeline: reverse chronological, with relative time display ("2 minutes ago")
- Empty state: friendly message when no tasks exist yet

**Verify:** Create a few tasks and run them via CLI, then check dashboard shows correct stats and timeline

**Commit:** `feat: dashboard page with stats and timeline`

---

### Task 13: Task List + Create/Edit Pages

**Files:**
- Create: `web/src/views/tasks/TaskList.vue`
- Create: `web/src/views/tasks/TaskForm.vue`
- Create: `web/src/views/tasks/TaskDetail.vue`
- Create: `web/src/api/task.js`

**What to build:**
**TaskList**: Table with columns (name, type tag, schedule, last status, last run time, enabled switch, actions). Filters: type dropdown, status dropdown, search by name. Actions: edit, run now, delete (with confirm). Pagination. Enabled toggle calls enable/disable API inline.

**TaskForm**: Create/Edit form. Type selector (remind/script/agent) controls which fields show. Common fields: name, schedule_type selector, schedule_expr (with cron helper tooltip), timeout, retry_count, retry_interval, notify_on checkboxes, notify_channel dropdown. Type-specific: remind → target textarea for message; script → target input for script path + args JSON editor; agent → target input for agent instruction. Cron helper: show human-readable description of cron expression as user types (use `cronstrue` npm package or similar).

**TaskDetail**: Task info card + execution history table below. History table: status, triggered_by, started_at, duration, with link to run detail.

**Key details:**
- schedule_expr validation on frontend: try parse cron with a JS lib, show error if invalid
- Type badges: remind=blue, script=green, agent=purple
- Run now button: show confirmation, then call API, show success message

**Verify:** Create task via form → appears in list → toggle enabled → run now → check detail page shows run history

**Commit:** `feat: task list, create/edit, and detail pages`

---

### Task 14: Run Detail + User Management + Notification Pages

**Files:**
- Create: `web/src/views/runs/RunDetail.vue`
- Create: `web/src/views/users/UserList.vue`
- Create: `web/src/views/users/UserForm.vue`
- Create: `web/src/views/notifications/NotificationList.vue`
- Create: `web/src/api/run.js`
- Create: `web/src/api/user.js`
- Create: `web/src/api/notification.js`

**What to build:**
**RunDetail**: Shows task run info (status, triggered_by, started_at, duration, exit_code) + output log viewer. Log viewer: monospace font, dark background (#1e1e1e), preserve whitespace, support basic ANSI color rendering (use `ansi-to-html` npm package). Show error section separately if present.

**UserList**: Admin-only page. Table with username, display_name, role tag, status tag, created_at, actions (edit/delete). Create button opens UserForm dialog.

**UserForm**: Dialog form for create/edit user. Fields: username, display_name, password (create only, or reset), role selector, status selector.

**NotificationList**: Table with task name, channel tag, status tag, created_at, payload preview. Filter by channel and status. Click row to expand and show full payload + response.

**Key details:**
- RunDetail output viewer: auto-scroll to bottom for long output
- UserList: only visible to admin role, router guard checks role
- Notification payload: show as formatted JSON in expandable row

**Verify:** Run a script task that produces output → check RunDetail page; create/edit users as admin; trigger a notification and check notification list

**Commit:** `feat: run detail, user management, and notification pages`

---

## Phase 5: Build + Polish

### Task 15: Build Pipeline + go:embed Integration

**Files:**
- Modify: `Makefile`
- Modify: `internal/api/router.go`
- Create: `web/dist/.gitkeep`

**What to build:**
Finalize `Makefile`: `make dev` runs backend + frontend concurrently, `make build` runs frontend build then Go build with embed, `make clean` removes artifacts. Ensure `go:embed` correctly serves the Vue SPA from the binary: all `/api/` routes go to gin handlers, all other routes serve static files from embedded filesystem with SPA fallback to `index.html`. Test the full production build: single binary serves both API and frontend.

**Key details:**
- `make build`: `cd web && npm ci && npm run build && cd .. && go build -ldflags "-X main.version=$(git describe)" -o bin/kronos ./cmd/kronos`
- Version injection via ldflags
- SPA fallback: gin `NoRoute` handler serves index.html for any non-API path
- Gzip middleware for static assets

**Verify:** `make build` → `./bin/kronos serve` → open `http://localhost:8360` in browser → full app works from single binary

**Commit:** `feat: production build pipeline with go:embed`

---

## Task Dependencies

```
Task 1 (skeleton) → Task 2 (models) → Task 3 (auth)
                                     → Task 4 (notifier)
                  Task 3 + 4 → Task 5 (executor) → Task 6 (scheduler)
                  Task 3 + 6 → Task 7 (API) → Task 8 (server)
                  Task 8 → Task 9 (CLI)
                  Task 8 → Task 10 (MCP)
                  Task 8 → Task 11 (Vue scaffold) → Task 12 (dashboard)
                                                  → Task 13 (tasks pages)
                         Task 12 + 13 → Task 14 (remaining pages)
                                      → Task 15 (build pipeline)
```

Tasks 3 and 4 can be parallelized. Tasks 9, 10, and 11 can be parallelized. Tasks 12 and 13 can be parallelized.
