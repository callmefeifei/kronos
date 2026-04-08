package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/pstrr/kronos/internal/executor"
	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/store"
)

// Scheduler manages scheduled task execution using robfig/cron.
type Scheduler struct {
	cron      *cron.Cron
	runner    *executor.Runner
	taskStore *store.TaskStore

	mu       sync.Mutex
	entries  map[int64]cron.EntryID // taskID → cron EntryID
	timers   map[int64]*time.Timer  // taskID → timer for "once" tasks
	sem      chan struct{}           // semaphore for concurrency control
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// New creates a Scheduler with the given concurrency limit.
func New(runner *executor.Runner, taskStore *store.TaskStore, maxConcurrent int) *Scheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	return &Scheduler{
		cron:      cron.New(cron.WithSeconds()),
		runner:    runner,
		taskStore: taskStore,
		entries:   make(map[int64]cron.EntryID),
		timers:    make(map[int64]*time.Timer),
		sem:       make(chan struct{}, maxConcurrent),
	}
}

// Start loads all enabled tasks from the store and begins scheduling.
func (s *Scheduler) Start() error {
	tasks, err := s.taskStore.ListEnabled()
	if err != nil {
		return fmt.Errorf("load enabled tasks: %w", err)
	}

	for i := range tasks {
		if err := s.addTask(&tasks[i]); err != nil {
			slog.Error("failed to schedule task", "task_id", tasks[i].ID, "name", tasks[i].Name, "error", err)
		}
	}

	s.cron.Start()
	slog.Info("scheduler started", "tasks_loaded", len(tasks))
	return nil
}

// Stop gracefully shuts down the scheduler, waiting up to 30s for running tasks.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		slog.Info("scheduler stopping")

		// Stop the cron scheduler (waits for its own goroutines).
		ctx := s.cron.Stop()
		<-ctx.Done()

		// Cancel all pending once-timers.
		s.mu.Lock()
		for id, t := range s.timers {
			t.Stop()
			delete(s.timers, id)
		}
		s.mu.Unlock()

		// Wait for running task goroutines with a 30s deadline.
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			slog.Info("scheduler stopped gracefully")
		case <-time.After(30 * time.Second):
			slog.Warn("scheduler stop timed out after 30s, some tasks may still be running")
		}
	})
}

// AddTask registers a task with the scheduler. Safe for concurrent use.
func (s *Scheduler) AddTask(task *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addTask(task)
}

// RemoveTask unregisters a task from the scheduler.
func (s *Scheduler) RemoveTask(taskID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeTask(taskID)
}

// UpdateTask removes an old entry and adds the updated task.
func (s *Scheduler) UpdateTask(task *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeTask(task.ID)
	if task.Enabled {
		return s.addTask(task)
	}
	return nil
}

// RunNow executes a task immediately, bypassing the schedule but still
// respecting the concurrency semaphore.
func (s *Scheduler) RunNow(taskID int64) error {
	task, err := s.taskStore.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("get task %d: %w", taskID, err)
	}
	s.executeTask(task, "manual")
	return nil
}

// addTask schedules a single task. Caller must hold s.mu.
func (s *Scheduler) addTask(task *model.Task) error {
	switch task.ScheduleType {
	case "cron":
		return s.addCronTask(task)
	case "interval":
		return s.addIntervalTask(task)
	case "once":
		return s.addOnceTask(task)
	default:
		return fmt.Errorf("unknown schedule_type %q for task %d", task.ScheduleType, task.ID)
	}
}

// addCronTask registers a cron-expression task.
func (s *Scheduler) addCronTask(task *model.Task) error {
	taskCopy := *task
	entryID, err := s.cron.AddFunc(task.ScheduleExpr, func() {
		s.executeAndUpdateNext(&taskCopy)
	})
	if err != nil {
		return fmt.Errorf("add cron entry for task %d: %w", task.ID, err)
	}
	s.entries[task.ID] = entryID
	s.updateNextRunAt(task, &entryID)
	slog.Debug("scheduled cron task", "task_id", task.ID, "expr", task.ScheduleExpr)
	return nil
}

// addIntervalTask converts an interval to @every syntax and registers it.
func (s *Scheduler) addIntervalTask(task *model.Task) error {
	expr := "@every " + task.ScheduleExpr
	taskCopy := *task
	entryID, err := s.cron.AddFunc(expr, func() {
		s.executeAndUpdateNext(&taskCopy)
	})
	if err != nil {
		return fmt.Errorf("add interval entry for task %d: %w", task.ID, err)
	}
	s.entries[task.ID] = entryID
	s.updateNextRunAt(task, &entryID)
	slog.Debug("scheduled interval task", "task_id", task.ID, "interval", task.ScheduleExpr)
	return nil
}

// addOnceTask schedules a one-time execution using time.AfterFunc.
func (s *Scheduler) addOnceTask(task *model.Task) error {
	target, err := time.Parse(time.RFC3339, task.ScheduleExpr)
	if err != nil {
		return fmt.Errorf("parse once target time for task %d: %w", task.ID, err)
	}

	taskCopy := *task
	delay := time.Until(target)

	if delay <= 0 {
		// Target is in the past.
		if task.LastStatus == "" {
			// Never ran — execute immediately.
			slog.Info("once task target in past, executing immediately", "task_id", task.ID)
			go s.executeTask(&taskCopy, "scheduler")
			return nil
		}
		// Already ran — skip.
		slog.Debug("once task already executed, skipping", "task_id", task.ID)
		return nil
	}

	// Schedule for the future.
	nextRun := target
	task.NextRunAt = &nextRun
	if err := s.taskStore.Update(task); err != nil {
		slog.Error("failed to update next_run_at for once task", "task_id", task.ID, "error", err)
	}

	timer := time.AfterFunc(delay, func() {
		s.executeTask(&taskCopy, "scheduler")
		// Clear next_run_at after execution since it won't run again.
		taskCopy.NextRunAt = nil
		if err := s.taskStore.Update(&taskCopy); err != nil {
			slog.Error("failed to clear next_run_at after once task", "task_id", taskCopy.ID, "error", err)
		}
		s.mu.Lock()
		delete(s.timers, taskCopy.ID)
		s.mu.Unlock()
	})
	s.timers[task.ID] = timer
	slog.Debug("scheduled once task", "task_id", task.ID, "target", target, "delay", delay)
	return nil
}

// removeTask unregisters a task. Caller must hold s.mu.
func (s *Scheduler) removeTask(taskID int64) {
	if entryID, ok := s.entries[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, taskID)
	}
	if timer, ok := s.timers[taskID]; ok {
		timer.Stop()
		delete(s.timers, taskID)
	}
}

// executeTask acquires the semaphore and runs the task in a goroutine.
func (s *Scheduler) executeTask(task *model.Task, triggeredBy string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Acquire semaphore.
		s.sem <- struct{}{}
		defer func() { <-s.sem }()

		slog.Info("executing task", "task_id", task.ID, "name", task.Name, "triggered_by", triggeredBy)

		// Reload the task from store to get fresh state.
		fresh, err := s.taskStore.GetByID(task.ID)
		if err != nil {
			slog.Error("failed to reload task before execution", "task_id", task.ID, "error", err)
			return
		}

		ctx := context.Background()
		s.runner.Run(ctx, fresh, triggeredBy)
	}()
}

// executeAndUpdateNext runs the task and then computes the next run time.
func (s *Scheduler) executeAndUpdateNext(task *model.Task) {
	s.executeTask(task, "scheduler")

	// Update next_run_at from the cron entry's next scheduled time.
	s.mu.Lock()
	entryID, ok := s.entries[task.ID]
	s.mu.Unlock()
	if ok {
		s.updateNextRunAt(task, &entryID)
	}
}

// updateNextRunAt computes and persists the next run time from the cron entry.
func (s *Scheduler) updateNextRunAt(task *model.Task, entryID *cron.EntryID) {
	if entryID == nil {
		return
	}
	entry := s.cron.Entry(*entryID)
	if entry.ID == 0 {
		return
	}
	next := entry.Next
	if next.IsZero() {
		return
	}
	task.NextRunAt = &next
	if err := s.taskStore.Update(task); err != nil {
		slog.Error("failed to update next_run_at", "task_id", task.ID, "error", err)
	}
}
