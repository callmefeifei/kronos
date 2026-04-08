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
		mcp.WithString("schedule_expr", mcp.Description("Schedule expression (cron expr, duration, or RFC3339 time)"), mcp.Required()),
		mcp.WithString("target", mcp.Description("Execution target (script path, reminder text, etc.)"), mcp.Required()),
		mcp.WithNumber("timeout", mcp.Description("Timeout in seconds (default 300)")),
		mcp.WithNumber("retry_count", mcp.Description("Number of retries (default 3)")),
		mcp.WithBoolean("enabled", mcp.Description("Whether the task is enabled (default true)")),
	)
}

func updateTaskTool() mcp.Tool {
	return mcp.NewTool("update_task",
		mcp.WithDescription("Update an existing scheduled task (partial update, only provided fields are changed)"),
		mcp.WithNumber("task_id", mcp.Description("ID of the task to update"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name")),
		mcp.WithString("type", mcp.Description("Task type: remind, script, agent"), mcp.Enum("remind", "script", "agent")),
		mcp.WithString("schedule_type", mcp.Description("Schedule type: cron, interval, once"), mcp.Enum("cron", "interval", "once")),
		mcp.WithString("schedule_expr", mcp.Description("Schedule expression (cron expr, duration, or RFC3339 time)")),
		mcp.WithString("target", mcp.Description("Execution target (script path, reminder text, etc.)")),
		mcp.WithNumber("timeout", mcp.Description("Timeout in seconds")),
		mcp.WithNumber("retry_count", mcp.Description("Number of retries")),
		mcp.WithBoolean("enabled", mcp.Description("Whether the task is enabled")),
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
		mcp.WithDescription("Poll and drain pending agent tasks that need execution. Returns tasks with prompts that the calling agent should execute. Tasks are removed from the queue once returned."),
	)
}
