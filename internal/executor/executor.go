package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/notifier"
	"github.com/pstrr/kronos/internal/store"
)

// maxOutputBytes is the maximum output size kept per task run (64 KB).
const maxOutputBytes = 64 * 1024

// RunResult holds the outcome of a single execution attempt.
type RunResult struct {
	Status   string        // success, failed, timeout
	ExitCode int           // meaningful for script type
	Output   string        // combined stdout+stderr (truncated)
	Error    string        // error message if any
	Duration time.Duration // wall-clock duration
}

// Executor is the interface every task-type executor must implement.
type Executor interface {
	Execute(ctx context.Context, task *model.Task) *RunResult
}

// Dispatch returns the appropriate Executor for a given task type.
func Dispatch(taskType string) (Executor, error) {
	switch taskType {
	case "remind":
		return &RemindExecutor{}, nil
	case "script":
		return &ScriptExecutor{}, nil
	case "agent":
		return &AgentExecutor{}, nil
	default:
		return nil, fmt.Errorf("unknown task type: %s", taskType)
	}
}

// Runner orchestrates task execution including run records, retries, and notifications.
type Runner struct {
	TaskStore    *store.TaskStore
	TaskRunStore *store.TaskRunStore
	Notifier     *notifier.Manager
}

// NewRunner creates a Runner with the required dependencies.
func NewRunner(ts *store.TaskStore, trs *store.TaskRunStore, n *notifier.Manager) *Runner {
	return &Runner{
		TaskStore:    ts,
		TaskRunStore: trs,
		Notifier:     n,
	}
}

// Run executes a task end-to-end: creates task_run records, handles retries,
// sends notifications per notify_on config, and updates the task's last_run_at/last_status.
func (r *Runner) Run(ctx context.Context, task *model.Task, triggeredBy string) {
	executor, err := Dispatch(task.Type)
	if err != nil {
		slog.Error("cannot dispatch executor", "task_id", task.ID, "type", task.Type, "error", err)
		return
	}

	maxAttempts := task.RetryCount + 1 // first attempt + retries
	var lastResult *RunResult

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Sleep before retry (not before the first attempt).
		if attempt > 0 {
			sleepDuration := time.Duration(task.RetryInterval) * time.Second
			slog.Info("retrying task", "task_id", task.ID, "attempt", attempt, "sleep", sleepDuration)
			select {
			case <-ctx.Done():
				slog.Warn("context cancelled before retry", "task_id", task.ID)
				return
			case <-time.After(sleepDuration):
			}
		}

		// Create task_run record.
		now := time.Now()
		run := &model.TaskRun{
			TaskID:       task.ID,
			TriggeredBy:  triggeredBy,
			Status:       "running",
			StartedAt:    now,
			RetryAttempt: attempt,
		}
		if err := r.TaskRunStore.Create(run); err != nil {
			slog.Error("failed to create task_run", "task_id", task.ID, "error", err)
			return
		}

		// Build execution context with timeout.
		timeout := time.Duration(task.Timeout) * time.Second
		execCtx, cancel := context.WithTimeout(ctx, timeout)

		// Execute.
		result := executor.Execute(execCtx, task)
		cancel()

		lastResult = result

		// Update task_run with result.
		finished := time.Now()
		run.FinishedAt = &finished
		run.DurationMs = int(result.Duration.Milliseconds())
		run.Status = result.Status
		run.Output = result.Output
		run.Error = result.Error
		if task.Type == "script" {
			exitCode := result.ExitCode
			run.ExitCode = &exitCode
		}

		if err := r.TaskRunStore.Update(run); err != nil {
			slog.Error("failed to update task_run", "task_run_id", run.ID, "error", err)
		}

		// If success, no need to retry.
		if result.Status == "success" {
			r.maybeNotify(task, run, "success")
			break
		}

		// Last attempt failed — send failure notification.
		if attempt == maxAttempts-1 {
			r.maybeNotify(task, run, "fail")
		}
	}

	// Update task's last_run_at and last_status.
	now := time.Now()
	task.LastRunAt = &now
	if lastResult != nil {
		task.LastStatus = lastResult.Status
	}
	if err := r.TaskStore.Update(task); err != nil {
		slog.Error("failed to update task last_run info", "task_id", task.ID, "error", err)
	}
}

// maybeNotify checks the task's notify_on config and sends a notification if appropriate.
func (r *Runner) maybeNotify(task *model.Task, run *model.TaskRun, status string) {
	if r.Notifier == nil || !r.Notifier.HasChannels() {
		return
	}

	// Parse notify_on JSON: {"success": bool, "fail": bool}
	notifyOn := map[string]bool{}
	if task.NotifyOn != nil {
		_ = json.Unmarshal(task.NotifyOn, &notifyOn)
	}

	shouldNotify, ok := notifyOn[status]
	if !ok || !shouldNotify {
		return
	}

	n := &notifier.Notification{
		TaskID:    task.ID,
		TaskRunID: run.ID,
		TaskName:  task.Name,
		Status:    status,
		Output:    run.Output,
		Error:     run.Error,
		Timestamp: time.Now(),
	}

	if task.NotifyChannel != "" {
		r.Notifier.SendTo(task.NotifyChannel, n)
	} else {
		r.Notifier.Send(n)
	}
}

// truncateOutput keeps the last maxOutputBytes of output if it exceeds the limit.
func truncateOutput(s string) string {
	if len(s) <= maxOutputBytes {
		return s
	}
	return "[...truncated...]\n" + s[len(s)-maxOutputBytes:]
}
