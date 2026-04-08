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

1. On startup, probe `http://localhost:{api_port}/health`
2. **Probe succeeds → Proxy mode**: all tool handlers call daemon REST API via HTTP (reuse JWT auth)
3. **Probe fails → Embedded mode**: open SQLite directly, create stores + scheduler, handlers operate on DB
4. Log current mode to stderr (MCP protocol uses only stdin/stdout)

### Rationale

- Proxy mode: no SQLite WAL lock conflicts, scheduler state consistent, zero duplication
- Embedded mode: works standalone when daemon isn't running (e.g., one-off queries)
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
| `server_status` | Server status (version, task counts, connection mode) | `GET /api/v1/status` |

### `update_task` Parameters

- `task_id` (required) — ID of the task to update
- `name`, `type`, `schedule_type`, `schedule_expr`, `target` — optional, only provided fields are updated
- `timeout`, `retry_count`, `enabled` — optional

### `delete_task` Parameters

- `task_id` (required) — ID of the task to delete

### `server_status` Parameters

None. Returns version, uptime, total tasks, enabled tasks, connection mode (proxy/embedded).

## 3. `kronos setup mcp` Auto-Configuration

### Command

```bash
kronos setup mcp [--platform <name>] [--print]
```

### Supported Platforms

| Platform | Config Path | Key |
|----------|------------|-----|
| `claude` | `~/.claude/settings.json` | `mcpServers` |
| `cursor` | `<project>/.cursor/mcp.json` | `mcpServers` |
| `opencode` | `~/.config/opencode/config.json` | `mcpServers` |
| `windsurf` | `~/.windsurf/mcp.json` | `mcpServers` |

### Behavior

- No `--platform` flag → interactive selection (list detected platforms)
- `--print` → output JSON to stdout, don't write files
- Auto-detect kronos binary path via `os.Executable()`
- Prompt before overwriting existing configuration
- Validate JSON integrity after write

### Generated Config

```json
{
  "kronos": {
    "command": "/path/to/kronos",
    "args": ["mcp", "--config", "~/.kronos/kronos.yaml"]
  }
}
```

## 4. Internal Architecture

### File Structure

```
internal/mcp/
├── server.go          # Existing SSE server (unchanged)
├── tools.go           # Shared tool definitions (extracted from server.go)
├── handler_direct.go  # Direct store/scheduler handlers (existing logic, moved)
├── handler_proxy.go   # HTTP API proxy handlers (new)
└── stdio.go           # Stdio server entry point (new)

internal/apiclient/
└── client.go          # Shared HTTP client + JWT generation (extracted from cmd/commands/helpers.go)

cmd/commands/
├── mcp.go             # `kronos mcp` command (new)
└── setup.go           # `kronos setup mcp` command (new)
```

### Handler Interface

Both direct and proxy handlers implement the same MCP tool handler signature:

```go
func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
```

The stdio entry point selects which handler set to register based on the probe result.

### Shared API Client

Extract from `cmd/commands/helpers.go`:
- `generateAdminJWT(secret string) (string, error)` → `internal/apiclient/`
- HTTP client with auth header → `internal/apiclient/Client`

This avoids `internal/mcp` depending on `cmd/commands`.

## 5. Non-Goals

- Changing the existing SSE server behavior
- Adding new notification channels
- Implementing the agent executor
- Authentication changes (stdio inherits config-based token/JWT)
