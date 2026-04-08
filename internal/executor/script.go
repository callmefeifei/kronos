package executor

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pstrr/kronos/internal/model"
)

// ScriptExecutor runs a script or command specified in the task's target field.
type ScriptExecutor struct{}

// Execute runs the task target as a script/command, captures combined output,
// and respects the context timeout.
func (e *ScriptExecutor) Execute(ctx context.Context, task *model.Task) *RunResult {
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

// buildCommand creates the appropriate exec.Cmd based on the target's file extension.
// - .py files are run via python3
// - .sh files or no extension are run via sh -c
// - everything else is executed directly
func buildCommand(ctx context.Context, target string) *exec.Cmd {
	ext := strings.ToLower(filepath.Ext(target))

	switch ext {
	case ".py":
		return exec.CommandContext(ctx, "python3", target)
	case ".sh":
		return exec.CommandContext(ctx, "sh", "-c", target)
	default:
		// No extension → treat as shell command; otherwise execute directly.
		if ext == "" {
			return exec.CommandContext(ctx, "sh", "-c", target)
		}
		return exec.CommandContext(ctx, target)
	}
}
