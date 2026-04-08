# MCP Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Kronos accessible to AI coding tools (Claude Code, Cursor, OpenCode, Windsurf) via MCP stdio transport with hybrid proxy/embedded mode.

**Architecture:** Add a `kronos mcp` stdio subcommand that probes the daemon API and selects proxy or embedded handler mode. Extract shared API client from CLI helpers. Add `kronos setup mcp` for multi-platform auto-configuration.

**Tech Stack:** Go, mark3labs/mcp-go (StdioServer), cobra, mcp-go tool definitions

**Spec:** `docs/superpowers/specs/2026-04-09-mcp-skill-design.md`

---

### Task 1: Extract shared API client to `internal/apiclient`

**Files:**
- Create: `internal/apiclient/client.go`
- Modify: `cmd/commands/helpers.go`
- Modify: `cmd/commands/task.go`

**What to build:**
Extract `generateAdminJWT` from `cmd/commands/helpers.go` and the `apiClient` struct from `cmd/commands/task.go` into a new `internal/apiclient` package. The new `apiclient.Client` should expose `Get`, `Post`, `Put`, `Delete` methods returning `(map[string]interface{}, error)`. Constructor: `New(baseURL, jwtSecret string) (*Client, error)` — generates JWT internally. Refactor `cmd/commands/task.go` and `helpers.go` to use the new package, removing duplicated code.

**Key details:**
- Keep `loadConfigForCLI` in `cmd/commands/helpers.go` (it depends on cobra's `cfgFile` var)
- `apiClient` in task.go currently uses `serverURL` package var — the new client takes `baseURL` as explicit param
- Also move `checkResponse`, `extractData`, `extractPagedItems` response helpers to apiclient
- Add a `put` method (currently missing in task.go, needed for `update_task`)

**Verify:** `go build ./...` compiles. Existing CLI commands (`kronos task list`, etc.) still work against a running daemon.

**Commit:** `refactor: extract shared API client to internal/apiclient`

---

### Task 2: Extract MCP tool definitions and direct handlers

**Files:**
- Create: `internal/mcp/tools.go`
- Create: `internal/mcp/handler_direct.go`
- Modify: `internal/mcp/server.go`

**What to build:**
Split `internal/mcp/server.go` into three files. `tools.go` holds all tool definition functions (`listTasksTool()`, `createTaskTool()`, etc.) plus the new `updateTaskTool()`, `deleteTaskTool()`, `serverStatusTool()` definitions. `handler_direct.go` holds a `DirectHandler` struct (with taskStore, taskRunStore, scheduler fields) and all handler methods. `server.go` retains `Server` struct, `Start`, `Shutdown`, auth middleware, but `registerTools` now creates a `DirectHandler` and registers its methods.

**Key details:**
- `DirectHandler` methods skip auth check (the SSE server's `Server.registerTools` wraps them with auth via the existing `checkAuth` pattern on the `Server` level)
- Add new tool definitions: `updateTaskTool` mirrors `createTaskTool` fields but all optional except `task_id`; `deleteTaskTool` takes `task_id`; `serverStatusTool` takes no params
- Add `handleUpdateTask` on DirectHandler: partial update using provided fields, call `scheduler.UpdateTask` if schedule changed
- Add `handleDeleteTask`: delete from store, remove from scheduler
- Add `handleServerStatus`: return task counts from store + version from build vars
- Follow existing `jsonResult()` pattern for responses

**Verify:** `go build ./...`. Existing SSE MCP server still works.

**Commit:** `refactor: split MCP tools/handlers, add update_task/delete_task/server_status`

---

### Task 3: Add proxy handlers for MCP tools

**Files:**
- Create: `internal/mcp/handler_proxy.go`

**What to build:**
A `ProxyHandler` struct wrapping `*apiclient.Client` that implements all 10 tool handlers by translating MCP requests to HTTP API calls. Each handler extracts params from `mcp.CallToolRequest`, calls the appropriate REST endpoint, and returns the result as `mcp.CallToolResult`.

**Key details:**
- Handler mapping: `list_tasks` → `GET /api/v1/tasks`, `create_task` → `POST /api/v1/tasks`, `update_task` → `PUT /api/v1/tasks/:id`, `delete_task` → `DELETE /api/v1/tasks/:id`, `run_task` → `POST /api/v1/tasks/:id/run`, `get_task_status` → `GET /api/v1/tasks/:id`, `get_task_history` → `GET /api/v1/tasks/:id/runs`, `enable_task` → `POST /api/v1/tasks/:id/enable`, `disable_task` → `POST /api/v1/tasks/:id/disable`, `server_status` → `GET /api/v1/status`
- `server_status` in proxy mode also calls `GET /api/v1/tasks?size=1` to get total count, then enriches the status response with `mode: "proxy"`
- No MCP-level auth check — stdio is implicitly trusted
- API response format is `{"code":0,"message":"ok","data":...}` — proxy handlers extract `data` and return it as JSON tool result; on non-zero code, return `mcp.NewToolResultError`

**Verify:** `go build ./...`

**Commit:** `feat: add MCP proxy handlers for stdio transport`

---

### Task 4: Add stdio MCP server entry point

**Files:**
- Create: `internal/mcp/stdio.go`

**What to build:**
A `StartStdio(cfg *config.Config) error` function that implements the hybrid mode: probe daemon health endpoint, select handler set, create MCP server, and call `server.ServeStdio()`. Log mode selection to stderr.

**Key details:**
- Probe: `GET http://localhost:{cfg.Server.Port}/health` with 2s timeout. Success = proxy mode.
- Proxy mode: create `apiclient.New(baseURL, cfg.Auth.JWTSecret)`, create `ProxyHandler`, register all tools
- Embedded mode: open DB via `store.NewDatabase(cfg.Database)`, create stores, create `scheduler.New()` but do NOT call `sched.Start()` (no cron scheduling). Create `DirectHandler`, register all tools. On exit, close DB.
- Use `server.NewMCPServer("kronos", version, server.WithInstructions(...))` then `server.NewStdioServer(mcpSrv)` and call `srv.Listen(ctx, os.Stdin, os.Stdout)`
- Handle SIGINT/SIGTERM for clean shutdown (close DB in embedded mode)
- stderr logging: `fmt.Fprintf(os.Stderr, "kronos mcp: running in %s mode\n", mode)`

**Verify:** `go build ./...`

**Commit:** `feat: add MCP stdio server with hybrid proxy/embedded mode`

---

### Task 5: Add `kronos mcp` CLI command

**Files:**
- Create: `cmd/commands/mcp.go`
- Modify: `cmd/commands/root.go`

**What to build:**
A `newMCPCmd() *cobra.Command` for `kronos mcp` that loads config via `config.Load(cfgFile)` and calls `mcp.StartStdio(cfg)`. Register it in `root.go`.

**Key details:**
- Follow the `serve.go` pattern: load config, setup logging (redirect to stderr: log handler should write to stderr, not stdout, since stdout is MCP protocol)
- Config logging override: force `slog` handler to use `os.Stderr` regardless of config
- Simple command, no subcommands or extra flags beyond the global `--config`

**Verify:** `go build -o bin/kronos ./cmd/kronos && echo '{}' | ./bin/kronos mcp --config configs/kronos.yaml` should output MCP protocol initialization to stdout and mode info to stderr (will fail gracefully if no valid MCP input).

**Commit:** `feat: add kronos mcp command for stdio transport`

---

### Task 6: Add `kronos setup mcp` CLI command

**Files:**
- Create: `cmd/commands/setup.go`
- Modify: `cmd/commands/root.go`

**What to build:**
A `kronos setup mcp` command that generates and writes MCP client configuration for AI tool platforms. Supports `--platform` (claude/cursor/opencode/windsurf), `--print` (stdout only), `--force` (overwrite existing). Without `--platform`, presents a numbered list for interactive selection.

**Key details:**
- Platform config paths: `claude` → `~/.claude.json`, `cursor` → `~/.cursor/mcp.json`, `opencode` → `~/.config/opencode/config.json`, `windsurf` → `~/.codeium/windsurf/mcp_config.json`
- All platforms use the same `mcpServers` key with `{"kronos": {"command": "<path>", "args": ["mcp", "--config", "~/.kronos/kronos.yaml"]}}`
- Detect kronos binary via `os.Executable()` + `filepath.EvalSymlinks()`
- Config file merge logic: read existing JSON if present, add/update `kronos` key under `mcpServers`, write back with 2-space indent. If file doesn't exist, create with full structure.
- If `kronos` key already exists and `--force` not set, print warning and skip
- Use `bufio.NewReader(os.Stdin)` for interactive selection (follow pattern in task.go's create command)
- Structure as `newSetupCmd()` parent with `newSetupMCPCmd()` subcommand, so future `kronos setup X` commands are easy to add

**Verify:** `go build -o bin/kronos ./cmd/kronos && ./bin/kronos setup mcp --print --platform claude` should output valid JSON config.

**Commit:** `feat: add kronos setup mcp for multi-platform auto-configuration`

---

### Task 7: Integration testing and cleanup

**Files:**
- Modify: `Makefile` (optional: add `make mcp-test` target)

**What to build:**
Manual integration test: build the binary, start the daemon, verify `kronos mcp` works in proxy mode (using a simple MCP client or echo test), stop daemon, verify fallback to embedded mode. Run `kronos setup mcp --print` for each platform. Ensure all existing functionality still works.

**Key details:**
- Verify: `make build` succeeds
- Verify: `./bin/kronos serve` starts normally (SSE MCP still works)
- Verify: `./bin/kronos setup mcp --print --platform claude` outputs valid JSON for each platform
- Verify: existing CLI commands (`task list`, `status`, etc.) still work
- Clean up any TODO comments or dead code from the refactoring

**Commit:** `chore: integration verification and cleanup`
