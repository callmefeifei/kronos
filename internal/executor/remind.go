package executor

import (
	"context"
	"time"

	"github.com/pstrr/kronos/internal/model"
)

// RemindExecutor sends the task's target as a reminder message.
// It always returns success since the actual notification delivery
// is handled by the Runner's notify_on mechanism.
type RemindExecutor struct{}

// Execute delivers a remind-type task. The target field is the message content.
func (e *RemindExecutor) Execute(_ context.Context, task *model.Task, _ int64) *RunResult {
	start := time.Now()
	return &RunResult{
		Status:   "success",
		ExitCode: 0,
		Output:   task.Target,
		Duration: time.Since(start),
	}
}
