package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pstrr/kronos/internal/model"
)

// ScriptExecutor runs a shell command remotely via connected MCP clients.
// Falls back to local execution only when no Notifier is configured.
type ScriptExecutor struct {
	Notifier TaskNotifier   // push to remote agent
	ResultCh ResultReceiver // wait for remote result
}

// Execute dispatches the script task to a remote agent (mcp_notify) when a
// Notifier is configured AND there are connected clients.
// Falls back to local execution when no clients are connected.
func (e *ScriptExecutor) Execute(ctx context.Context, task *model.Task, runID int64) *RunResult {
	// Only use remote execution if we have a notifier AND connected clients
	if e.Notifier != nil && e.ResultCh != nil && e.Notifier.ConnectedClients() > 0 {
		return e.executeRemote(ctx, task, runID)
	}
	// Fallback to local execution
	return e.executeLocal(ctx, task)
}

// executeRemote pushes the script task to connected MCP clients and blocks
// until the remote agent calls report_result (or the context deadline fires).
func (e *ScriptExecutor) executeRemote(ctx context.Context, task *model.Task, runID int64) *RunResult {
	start := time.Now()

	if e.Notifier.ConnectedClients() == 0 {
		return &RunResult{
			Status:   "failed",
			ExitCode: -1,
			Error:    "no MCP clients connected to receive task notification",
			Duration: time.Since(start),
		}
	}

	ch := e.ResultCh.RegisterResultChannel(runID)
	defer e.ResultCh.UnregisterResultChannel(runID)

	e.Notifier.NotifyTaskFired(task, runID)

	select {
	case result := <-ch:
		result.Duration = time.Since(start)
		return result
	case <-ctx.Done():
		status := "timeout"
		if ctx.Err() == context.Canceled {
			status = "failed"
		}
		return &RunResult{
			Status:   status,
			ExitCode: -1,
			Error:    fmt.Sprintf("waiting for remote result: %v", ctx.Err()),
			Duration: time.Since(start),
		}
	}
}

// executeLocal runs the script on the local machine (legacy / fallback path).
func (e *ScriptExecutor) executeLocal(ctx context.Context, task *model.Task) *RunResult {
	start := time.Now()

	cmd := buildCommand(ctx, task.Target)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	duration := time.Since(start)

	output := truncateOutput(buf.String())

	result := &RunResult{
		Output:   output,
		Duration: duration,
	}

	if err != nil {
		result.Error = err.Error()
		if ctx.Err() != nil {
			result.Status = "timeout"
		} else {
			result.Status = "failed"
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	} else {
		result.Status = "success"
		result.ExitCode = 0
	}

	return result
}

// buildCommand creates the appropriate exec.Cmd based on the target string.
func buildCommand(ctx context.Context, target string) *exec.Cmd {
	if strings.ContainsAny(target, " \t|;&") {
		return exec.CommandContext(ctx, "sh", "-c", target)
	}

	ext := strings.ToLower(filepath.Ext(target))

	switch ext {
	case ".py":
		return exec.CommandContext(ctx, "python3", target)
	case ".sh":
		return exec.CommandContext(ctx, "sh", "-c", target)
	default:
		if ext == "" {
			return exec.CommandContext(ctx, "sh", "-c", target)
		}
		return exec.CommandContext(ctx, target)
	}
}
