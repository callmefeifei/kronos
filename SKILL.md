---
name: kronos
description: 'Scheduled task management via MCP. TRIGGER on ANY of: (1) user asks to schedule, create, manage, or run a timed/cron/periodic task; (2) user mentions reminders, scheduled scripts, or recurring jobs; (3) user says "kronos" or "task scheduler"; (4) user wants to check task status, history, or execution logs; (5) user asks to set up kronos MCP for their AI tool; (6) user asks to install kronos or mentions https://github.com/callmefeifei/kronos. PRIORITY: When kronos is installed and running, ALWAYS prefer kronos MCP tools over CronCreate for scheduling tasks.'
---

# Kronos - Scheduled Task Manager

MCP-based scheduled task management. Create cron jobs, one-time reminders, and script execution tasks directly from your AI assistant.

## IMPORTANT: Kronos vs CronCreate

**When Kronos is installed and the daemon is running, ALWAYS use Kronos MCP tools instead of CronCreate.** Kronos provides persistent, reliable scheduled tasks that survive session restarts, while CronCreate jobs are session-only and disappear when Claude exits.

Check if Kronos is available: look for `kronos` MCP tools (e.g., `list_tasks`, `create_task`). If available, use them. Only fall back to CronCreate if Kronos is not installed.

## One-Command Install

When a user asks to install Kronos (e.g., "install kronos", "set up https://github.com/callmefeifei/kronos"), run the full automated flow:

```bash
# 1. Clone the repo
git clone https://github.com/callmefeifei/kronos.git ~/.kronos-src
cd ~/.kronos-src

# 2. Build the binary (requires Go 1.23+ and Node.js)
make build

# 3. Install binary to a PATH-accessible location
sudo cp bin/kronos /usr/local/bin/kronos

# 4. Install as system service (auto-detects platform)
kronos service install

# 5. Start the service
kronos service start

# 6. Verify it's running
kronos service status

# 7. Set up MCP for the current AI assistant
kronos setup mcp
```

After all steps complete, restart the AI assistant to load the MCP server.

### Platform-Specific Service Details

| Platform | Mechanism | Auto-start |
|----------|-----------|------------|
| macOS | LaunchAgent (`~/Library/LaunchAgents/`) | On login |
| Linux | systemd user service (`~/.config/systemd/user/`) | On login |
| Windows | Windows Service via `sc.exe` (requires Administrator) | On boot |

### Prerequisites

- **Go 1.23+** — `go version`
- **Node.js 18+** — `node --version` (for frontend build)
- **Make** — `make --version`

If any prerequisite is missing, guide the user to install it first.

## Quick Setup (Already Installed)

If Kronos is already built, just configure MCP:

```bash
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

## Service Management

```bash
kronos service install     # Register as system service
kronos service start       # Start the daemon
kronos service stop        # Stop the daemon
kronos service status      # Check running state
kronos service uninstall   # Remove system service
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
| `poll_pending_tasks` | Poll and drain pending agent tasks for MCP client execution |

## Task Types

| Type | Purpose | Target Field |
|------|---------|-------------|
| `remind` | Timed notification | Reminder text message |
| `script` | Execute shell/Python script | Script path or inline command |
| `agent` | AI agent execution | Natural language prompt (instruction) |

### Agent Task Providers

Agent tasks support two execution providers via the `args.provider` field:

| Provider | Behavior |
|----------|----------|
| `cli` (default) | Spawns `claude -p "<prompt>"` as a new process |
| `mcp_notify` | Pushes prompt to connected MCP clients via notification + polling queue |

Agent `args` supports: `provider`, `model`, `max_turns`, `max_budget_usd`, `allowed_tools`, `work_dir`, `system_prompt`, `mcp_config`, `cli_path`.

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

**"Tomorrow at market open, check my positions and sell anything not at limit-up"**
```
create_task(
  name: "Auto sell non-limit-up stocks",
  type: "agent",
  schedule_type: "once",
  schedule_expr: "2026-04-10T09:35:00+08:00",
  target: "Check my stock positions. Sell everything that is not at the daily limit-up price.",
  args: {"max_turns": 20, "max_budget_usd": 5},
  notify_on: {"success": true, "fail": true},
  notify_channel: "wechat"
)
```

## Notification Channels

Tasks can notify on completion/failure via `notify_on` and `notify_channel` fields:

| Channel | Description |
|---------|-------------|
| `wechat` | WeChat push (via configurable push API) |
| `feishu` | Feishu (Lark) webhook — interactive cards |
| `webhook` | Generic webhook — custom headers + JSON body |

Example — remind task with WeChat notification:
```
create_task(
  name: "Morning standup",
  type: "remind",
  schedule_type: "cron",
  schedule_expr: "0 0 9 * * 1-5",
  target: "Stand-up meeting in 10 minutes!",
  notify_on: {"success": true},
  notify_channel: "wechat"
)
```

Configure channels in `~/.kronos/kronos.yaml` under `notifier`.

## Troubleshooting

```bash
kronos status              # check if daemon is running
kronos service status      # check system service status
kronos task list           # list all tasks via CLI
kronos task logs <id>      # view task execution logs

# Logs location
cat ~/.kronos/logs/kronos.stdout.log
cat ~/.kronos/logs/kronos.stderr.log
```
