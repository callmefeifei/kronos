# Kronos

Scheduled task management platform with MCP integration. Let your AI assistant manage cron jobs, reminders, and scripts — persistently.

## Why Kronos over CronCreate?

If you're using Claude Code, you might already know `CronCreate` — the built-in tool for scheduling prompts. Kronos solves the problems CronCreate can't:

| | CronCreate | Kronos |
|---|---|---|
| **Persistence** | Session-only. Close terminal = all jobs gone | System service. Survives reboots, session exits, everything |
| **Task types** | Only re-sends prompts to Claude | Shell scripts, reminders with webhook notifications, AI agents |
| **Visibility** | No UI. Forget what you scheduled? Good luck | Web dashboard at `localhost:8360` + CLI + MCP tools |
| **History** | None | Full execution logs, exit codes, durations, stdout/stderr |
| **Failure handling** | Silent | Retry, timeout control, failure notifications (Feishu/webhook) |
| **Precision** | Minute-level, with jitter | Second-level cron (6-field), intervals, one-time schedules |
| **Lifetime** | Auto-expires after 7 days | Runs forever until you stop it |
| **Cross-platform** | Claude Code only | Any MCP-compatible AI tool (Claude, Cursor, Windsurf, OpenCode) |
| **Management** | Create and delete, that's it | Create, update, enable/disable, run now, view history, filter |
| **Notifications** | None | Feishu cards, generic webhooks, configurable per-task |

**TL;DR:** CronCreate is a sticky note. Kronos is a crontab with a brain.

## Quick Install

```bash
# Clone & build (requires Go 1.23+ and Node.js 18+)
git clone https://github.com/callmefeifei/kronos.git ~/.kronos-src
cd ~/.kronos-src && make build

# Install binary
sudo cp bin/kronos /usr/local/bin/kronos

# Install & start as system service
kronos service install
kronos service start

# Set up MCP for your AI assistant
kronos setup mcp
```

Restart your AI assistant after setup. Done.

## Features

- **3 task types**: `remind` (notifications), `script` (shell/Python), `agent` (AI agent calls)
- **Flexible scheduling**: 6-field cron, fixed intervals (`@every 30m`), one-time (RFC3339)
- **Web UI**: Dashboard, task management, execution history at `localhost:8360`
- **MCP integration**: 10 tools for AI assistants to manage tasks directly
- **System service**: LaunchAgent (macOS), systemd (Linux), Windows Service
- **Notifications**: Feishu interactive cards, WeChat, generic webhooks
- **Health probes**: `/health` (liveness) and `/ready` (readiness with DB + scheduler checks)
- **Auto-cleanup**: Configurable retention policy for task run history (default 30 days)
- **Single binary**: Frontend embedded via `go:embed`, one file to deploy

## Service Management

```bash
kronos service install     # Register as system service
kronos service uninstall   # Remove service
kronos service start       # Start daemon
kronos service stop        # Stop daemon
kronos service status      # Check status
```

| Platform | Mechanism | Auto-start |
|----------|-----------|------------|
| macOS | LaunchAgent | On login |
| Linux | systemd user service | On login |
| Windows | Windows Service | On boot |

## MCP Tools

| Tool | Description |
|------|-------------|
| `list_tasks` | List tasks with filters |
| `create_task` | Create scheduled task |
| `update_task` | Update task fields |
| `delete_task` | Delete task |
| `run_task` | Trigger immediately |
| `get_task_status` | Task info + latest run |
| `get_task_history` | Execution history |
| `enable_task` / `disable_task` | Toggle task |
| `poll_pending_tasks` | Poll for tasks needing attention |
| `server_status` | Server health + counts |

## Configuration

Config auto-generated at `~/.kronos/kronos.yaml` on first run. Key settings:

```yaml
server:
  host: 0.0.0.0
  port: 8360

scheduler:
  max_concurrent: 10
  cleanup_retention_days: 30  # auto-delete task runs older than N days (0 = disabled)
  cleanup_interval_hours: 6

notifier:
  feishu:
    enabled: false
    webhook_url: ""
  wechat:
    enabled: false
    url: "https://api.ossec.cn/v1/send"
    token: ""
  webhook:
    enabled: false
    url: ""
```

## Health Checks

```
GET /health   → 200 {"status": "ok"}                        # liveness
GET /ready    → 200 {"status": "ready", "checks": {...}}     # readiness (DB + scheduler)
```

## Tech Stack

Go 1.23 · Gin · GORM · SQLite · robfig/cron · Vue 3 · Element Plus · Vite

## License

MIT
