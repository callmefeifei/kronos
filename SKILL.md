---
name: kronos
description: 'Scheduled task management via MCP. TRIGGER on ANY of: (1) user asks to schedule, create, manage, or run a timed/cron/periodic task; (2) user mentions reminders, scheduled scripts, or recurring jobs; (3) user says "kronos" or "task scheduler"; (4) user wants to check task status, history, or execution logs; (5) user asks to set up kronos MCP for their AI tool.'
---

# Kronos - Scheduled Task Manager

MCP-based scheduled task management. Create cron jobs, one-time reminders, and script execution tasks directly from your AI assistant.

## Quick Setup

Run this to install Kronos as an MCP tool in your current AI assistant:

```bash
# Build (if not already built)
cd /path/to/kronos && make build

# Auto-configure for your platform (interactive)
kronos setup mcp

# Or specify platform directly
kronos setup mcp --platform claude    # Claude Code
kronos setup mcp --platform cursor    # Cursor
kronos setup mcp --platform opencode  # OpenCode
kronos setup mcp --platform windsurf  # Windsurf

# Just print the config (don't write)
kronos setup mcp --print --platform claude
```

After setup, restart your AI assistant to load the MCP server.

## Starting the Daemon (Optional)

The MCP server works in two modes:
- **Proxy mode** (recommended): daemon running, MCP proxies through REST API
- **Embedded mode**: no daemon, MCP opens database directly

```bash
kronos serve                              # start daemon (port 8360)
kronos serve --config ~/.kronos/kronos.yaml  # with explicit config
```

## Available MCP Tools

| Tool | Description |
|------|-------------|
| `list_tasks` | List tasks with optional type/enabled filters |
| `create_task` | Create a new scheduled task |
| `update_task` | Update task fields (partial update) |
| `delete_task` | Delete a task |
| `run_task` | Trigger a task immediately |
| `get_task_status` | Get task info + latest run |
| `get_task_history` | Get execution history |
| `enable_task` | Enable a disabled task |
| `disable_task` | Disable a task |
| `server_status` | Server status and task counts |

## Task Types

| Type | Purpose | Target Field |
|------|---------|-------------|
| `remind` | Timed notification | Reminder text message |
| `script` | Execute shell/Python script | Script path or inline command |
| `agent` | Call external AI agent | Agent config (placeholder) |

## Schedule Types

| Type | Expression | Example |
|------|-----------|---------|
| `cron` | 6-field cron (with seconds) | `0 30 9 * * *` (daily 9:30) |
| `interval` | Fixed interval | `@every 30m`, `@every 1h` |
| `once` | One-time (RFC3339) | `2026-04-10T15:00:00+08:00` |

## Usage Examples

When the user asks to schedule a task, use `create_task`:

**"Remind me every morning at 9am to check emails"**
```
create_task(
  name: "Morning email reminder",
  type: "remind",
  schedule_type: "cron",
  schedule_expr: "0 0 9 * * *",
  target: "Check your emails!"
)
```

**"Run backup.sh every 6 hours"**
```
create_task(
  name: "Periodic backup",
  type: "script",
  schedule_type: "interval",
  schedule_expr: "@every 6h",
  target: "/home/user/backup.sh",
  timeout: 600
)
```

**"Run deploy.sh once at 3pm today"**
```
create_task(
  name: "Deploy at 3pm",
  type: "script",
  schedule_type: "once",
  schedule_expr: "2026-04-09T15:00:00+08:00",
  target: "/home/user/deploy.sh"
)
```

## Notification Channels

Tasks can notify on completion/failure via:
- **Feishu** (Lark) webhook — interactive cards
- **Generic webhook** — custom headers + JSON body

Configure in `~/.kronos/kronos.yaml` under `notifier`.

## Troubleshooting

```bash
kronos status              # check if daemon is running
kronos task list           # list all tasks via CLI
kronos task logs <id>      # view task execution logs
```
