package mcp

import "github.com/mark3labs/mcp-go/mcp"

// --- Tool Definitions ---

func listTasksTool() mcp.Tool {
	return mcp.NewTool("list_tasks",
		mcp.WithDescription("List scheduled tasks with optional filters"),
		mcp.WithString("type", mcp.Description("Filter by task type: remind, script, agent"), mcp.Enum("remind", "script", "agent")),
		mcp.WithBoolean("enabled", mcp.Description("Filter by enabled status")),
	)
}

func createTaskTool() mcp.Tool {
	return mcp.NewTool("create_task",
		mcp.WithDescription("Create a new scheduled task"),
		mcp.WithString("name", mcp.Description("Task name"), mcp.Required()),
		mcp.WithString("type", mcp.Description("Task type: remind, script, agent"), mcp.Required(), mcp.Enum("remind", "script", "agent")),
		mcp.WithString("schedule_type", mcp.Description("Schedule type: cron, interval, once"), mcp.Required(), mcp.Enum("cron", "interval", "once")),
		mcp.WithString("schedule_expr", mcp.Description("Schedule expression: cron expr (e.g. '0 13 * * *'), interval duration (e.g. '30m'), or datetime for once (RFC3339 '2006-01-02T15:04:05+08:00' or '2006-01-02 15:04:05')"), mcp.Required()),
		mcp.WithString("target", mcp.Description("Execution target (script path, reminder text, etc.)"), mcp.Required()),
		mcp.WithNumber("timeout", mcp.Description("Timeout in seconds (default 300)")),
		mcp.WithNumber("retry_count", mcp.Description("Number of retries (default 3)")),
		mcp.WithBoolean("enabled", mcp.Description("Whether the task is enabled (default true)")),
		mcp.WithString("notify_on", mcp.Description("When to send a notification: always (success+fail), success, fail, never. For remind tasks defaults to always."), mcp.Enum("always", "success", "fail", "never")),
		mcp.WithString("notify_channel", mcp.Description("Notification channel name, e.g. wechat, feishu (uses default channel if omitted)")),
	)
}

func updateTaskTool() mcp.Tool {
	return mcp.NewTool("update_task",
		mcp.WithDescription("Update an existing scheduled task (partial update, only provided fields are changed)"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task to update"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name")),
		mcp.WithString("type", mcp.Description("Task type: remind, script, agent"), mcp.Enum("remind", "script", "agent")),
		mcp.WithString("schedule_type", mcp.Description("Schedule type: cron, interval, once"), mcp.Enum("cron", "interval", "once")),
		mcp.WithString("schedule_expr", mcp.Description("Schedule expression: cron expr, interval duration, or datetime for once (RFC3339 or 'YYYY-MM-DD HH:MM:SS')")),
		mcp.WithString("target", mcp.Description("Execution target (script path, reminder text, etc.)")),
		mcp.WithNumber("timeout", mcp.Description("Timeout in seconds")),
		mcp.WithNumber("retry_count", mcp.Description("Number of retries")),
		mcp.WithBoolean("enabled", mcp.Description("Whether the task is enabled")),
		mcp.WithString("notify_on", mcp.Description("When to send a notification: always, success, fail, never"), mcp.Enum("always", "success", "fail", "never")),
		mcp.WithString("notify_channel", mcp.Description("Notification channel name, e.g. wechat, feishu")),
	)
}

func deleteTaskTool() mcp.Tool {
	return mcp.NewTool("delete_task",
		mcp.WithDescription("Delete a scheduled task"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task to delete"), mcp.Required()),
	)
}

func runTaskTool() mcp.Tool {
	return mcp.NewTool("run_task",
		mcp.WithDescription("Run a task immediately, bypassing its schedule"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task to run"), mcp.Required()),
	)
}

func getTaskStatusTool() mcp.Tool {
	return mcp.NewTool("get_task_status",
		mcp.WithDescription("Get task info combined with its latest run info"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task"), mcp.Required()),
	)
}

func getTaskHistoryTool() mcp.Tool {
	return mcp.NewTool("get_task_history",
		mcp.WithDescription("Get execution history for a task"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Number of runs to return (default 10)")),
	)
}

func enableTaskTool() mcp.Tool {
	return mcp.NewTool("enable_task",
		mcp.WithDescription("Enable a disabled task"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task to enable"), mcp.Required()),
	)
}

func disableTaskTool() mcp.Tool {
	return mcp.NewTool("disable_task",
		mcp.WithDescription("Disable an enabled task"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task to disable"), mcp.Required()),
	)
}

func serverStatusTool() mcp.Tool {
	return mcp.NewTool("server_status",
		mcp.WithDescription("Get server status including task counts and connection mode"),
	)
}

func pollPendingTasksTool() mcp.Tool {
	return mcp.NewTool("poll_pending_tasks",
		mcp.WithDescription("Poll and drain pending tasks that need remote execution. Returns tasks (type, target, run_id) that the calling agent should execute, then call report_result with the outcome."),
	)
}

func reportResultTool() mcp.Tool {
	return mcp.NewTool("report_result",
		mcp.WithDescription("Report the execution result of a task back to Kronos after the remote agent has finished running it. Must be called after receiving a task via poll_pending_tasks or kronos/task_fired notification."),
		mcp.WithNumber("run_id", mcp.Description("TaskRun ID received in the task notification"), mcp.Required()),
		mcp.WithString("status", mcp.Description("Execution outcome: success, failed, or timeout"), mcp.Required(), mcp.Enum("success", "failed", "timeout")),
		mcp.WithString("output", mcp.Description("Combined stdout/stderr output (max 64KB)")),
		mcp.WithString("error", mcp.Description("Error message if status is failed or timeout")),
		mcp.WithNumber("exit_code", mcp.Description("Process exit code (meaningful for script tasks)")),
	)
}
