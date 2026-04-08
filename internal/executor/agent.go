package executor

import (
	"context"
	"time"

	"github.com/pstrr/kronos/internal/model"
)

// AgentExecutor is a placeholder for AI agent task execution.
type AgentExecutor struct{}

// Execute returns a placeholder result indicating agent execution is not yet implemented.
func (e *AgentExecutor) Execute(_ context.Context, _ *model.Task) *RunResult {
	start := time.Now()
	return &RunResult{
		Status:   "failed",
		ExitCode: -1,
		Output:   "",
		Error:    "agent execution not yet implemented",
		Duration: time.Since(start),
	}
}
