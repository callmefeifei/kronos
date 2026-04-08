# Kronos MCP Skill Design

> Make Kronos accessible as a tool/skill for AI coding assistants (Claude Code, Cursor, OpenCode, Windsurf) via MCP stdio transport.

## Background

Kronos already has an MCP SSE server (port 8361) with 7 tools. However, modern AI coding tools prefer **stdio transport** — the tool spawns the process directly, no daemon pre-start required. We need to add stdio support and a setup command to auto-configure each platform.

## Scope

1. `kronos mcp` — stdio MCP server (hybrid: proxy or embedded)
2. `kronos setup mcp` — auto-configure AI tool platforms
3. New MCP tools: `update_task`, `delete_task`, `server_status`
4. Internal refactoring to support both SSE and stdio transports

## 1. `kronos mcp` Stdio Subcommand

### Command

```bash
kronos mcp [--config path]
```

### Hybrid Mode

1. On startup, load config via `config.Load(cfgFile)` to get `server.port` and `auth.jwt_secret`
2. Probe `http://localhost:{server.port}/health` with 2s timeout
3. **Probe succeeds → Proxy mode**: create `apiclient.Client` with jwt_secret, all tool handlers call daemon REST API
4. **Probe fails → Embedded mode**: open SQLite directly, create stores + scheduler, handlers operate on DB directly
5. Log current mode to stderr (MCP protocol uses only stdin/stdout)

### Authentication

- **Stdio transport is implicitly trusted** — the process runs locally, spawned by the AI tool. No MCP-level auth check needed.
- **Proxy mode**: the `apiclient.Client` generates admin JWT (same as CLI commands) to authenticate with the daemon API.
- **Embedded mode**: direct store access, no auth layer.
- The existing SSE server auth (`checkAuth`) remains unchanged.

### Embedded Mode Constraints

When running in embedded mode (daemon not available):
- **Full read-write access**: create, update, delete, enable, disable tasks all work
- **Scheduler runs in lightweight mode**: only handles `run_task` (manual trigger), does NOT start cron scheduling to avoid conflicts if daemon starts later
- **SQLite WAL mode**: concurrent reads are safe; writes are serialized by SQLite's internal locking. Since embedded mode is single-process, no lock conflicts occur when daemon is not running
- **If daemon starts while stdio is running in embedded mode**: SQLite WAL handles concurrent access safely for the duration, but user should restart the stdio process to switch to proxy mode for scheduler consistency

### Rationale

- Proxy mode: scheduler state consistent, zero duplication
- Embedded mode: works standalone when daemon isn't running (e.g., one-off queries, task management)
- Hybrid: best of both worlds, automatic detection

## 2. MCP Tool Enhancements

### Existing Tools (7)

- `list_tasks` — list with optional type/enabled filters
- `create_task` — create a new scheduled task
- `run_task` — trigger immediate execution
- `get_task_status` — task info + latest run
- `get_task_history` — execution history
- `enable_task` — enable a disabled task
- `disable_task` — disable an enabled task

### New Tools (3)

| Tool | Description | API Mapping |
|------|-------------|-------------|
| `update_task` | Update task fields (name, schedule, target, etc.) | `PUT /api/v1/tasks/:id` |
| `delete_task` | Delete a task permanently | `DELETE /api/v1/tasks/:id` |
| `server_status` | Server status (version, task counts, connection mode) | `GET /api/v1/status` + `GET /api/v1/tasks?size=1` for counts |

### `update_task` Parameters

- `task_id` (required) — ID of the task to update
- `name`, `type`, `schedule_type`, `schedule_expr`, `target` — optional, only provided fields are updated
- `timeout`, `retry_count`, `enabled` — optional

### `delete_task` Parameters

- `task_id` (required) — ID of the task to delete

### `server_status` Parameters

None. Returns:
- `mode`: "proxy" or "embedded"
- `version`: kronos version string
- In proxy mode: forwards daemon's `/api/v1/status` response (version, uptime, server_time, go_version) plus task counts from list query
- In embedded mode: returns version, mode, and task counts from direct DB query (no uptime since no daemon)

## 3. `kronos setup mcp` Auto-Configuration

### Command

```bash
kronos setup mcp [--platform <name>] [--print] [--force]
```

### Supported Platforms

| Platform | Config Path | Key |
|----------|------------|-----|
| `claude` | `~/.claude.json` | `mcpServers` |
| `cursor` | `~/.cursor/mcp.json` | `mcpServers` |
| `opencode` | `~/.config/opencode/config.json` | `mcpServers` |
| `windsurf` | `~/.codeium/windsurf/mcp_config.json` | `mcpServers` |

### Platform Detection

No `--platform` flag → show all 4 platforms as a numbered list, let user pick. No auto-detection magic — simple and predictable.

### Behavior

- `--print` → output JSON to stdout, don't write files
- Auto-detect kronos binary path via `os.Executable()`, resolve symlinks
- Config file already exists with `kronos` key → print warning and skip (use `--force` to overwrite)
- Config file exists without `kronos` key → merge into existing JSON
- Config file doesn't exist → create with proper structure
- Validate JSON integrity after write

### Generated Config

```json
{
  "mcpServers": {
    "kronos": {
      "command": "/path/to/kronos",
      "args": ["mcp", "--config", "~/.kronos/kronos.yaml"]
    }
  }
}
```

## 4. Internal Architecture

### File Structure

```
internal/mcp/
├── server.go          # SSE server: Server struct, Start/Shutdown, registerTools (refactored)
├── tools.go           # Shared tool definitions (extracted from server.go)
├── handler_direct.go  # Direct store/scheduler handlers (extracted from server.go)
├── handler_proxy.go   # HTTP API proxy handlers (new)
└── stdio.go           # Stdio server entry: probe, select handlers, ServeStdio (new)

internal/apiclient/
└── client.go          # Shared HTTP client + JWT generation (new, extracted from cmd/commands/helpers.go)

cmd/commands/
├── mcp.go             # `kronos mcp` command (new)
├── setup.go           # `kronos setup mcp` command (new)
└── helpers.go         # Refactored to use internal/apiclient (existing)
```

### Refactoring Notes

- `server.go` is refactored: tool definitions move to `tools.go`, handler implementations move to `handler_direct.go`. The `Server` struct, `Start`, `Shutdown`, and `registerTools` stay in `server.go`.
- `handler_direct.go` methods remain on the `Server` struct (or a `DirectHandler` struct that holds the same stores/scheduler).
- `handler_proxy.go` has a `ProxyHandler` struct wrapping `apiclient.Client`.
- Both handler types produce `func(ctx, req) (*CallToolResult, error)` — registered via a shared `registerAllTools(mcpSrv, handlers)` function.

### Shared API Client (`internal/apiclient`)

Config flow for `kronos mcp`:
1. `cmd/commands/mcp.go` calls `config.Load(cfgFile)` — gets full config
2. Extracts `cfg.Auth.JWTSecret` and `cfg.Server.Port`
3. Passes to `apiclient.New(baseURL, jwtSecret)` or to direct handler constructors
4. No dependency on `cmd/commands` from `internal/` packages — clean import graph

`cmd/commands/helpers.go` is refactored to call `apiclient.New()` instead of its own HTTP logic, eliminating duplication.

## 5. Non-Goals

- Changing the existing SSE server's external behavior
- Adding new notification channels
- Implementing the agent executor
- Starting cron scheduling in embedded mode
