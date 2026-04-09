package store

import (
	"time"

	"github.com/pstrr/kronos/internal/model"
	"gorm.io/gorm"
)

// TaskRunStore provides CRUD operations for task runs.
type TaskRunStore struct {
	db *gorm.DB
}

// NewTaskRunStore creates a new TaskRunStore.
func NewTaskRunStore(db *gorm.DB) *TaskRunStore {
	return &TaskRunStore{db: db}
}

// Create inserts a new task run.
func (s *TaskRunStore) Create(run *model.TaskRun) error {
	return s.db.Create(run).Error
}

// GetByID finds a task run by primary key.
func (s *TaskRunStore) GetByID(id int64) (*model.TaskRun, error) {
	var run model.TaskRun
	if err := s.db.First(&run, id).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// Update saves changes to an existing task run.
func (s *TaskRunStore) Update(run *model.TaskRun) error {
	return s.db.Save(run).Error
}

// Delete removes a task run by ID.
func (s *TaskRunStore) Delete(id int64) error {
	return s.db.Delete(&model.TaskRun{}, id).Error
}

// List returns a paginated list of all task runs.
func (s *TaskRunStore) List(page, size int) ([]model.TaskRun, int64, error) {
	var runs []model.TaskRun
	var total int64

	if err := s.db.Model(&model.TaskRun{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := s.db.Order("started_at DESC").Offset(offset).Limit(size).Find(&runs).Error; err != nil {
		return nil, 0, err
	}

	return runs, total, nil
}

// ListByTask returns a paginated list of task runs for a specific task.
func (s *TaskRunStore) ListByTask(taskID int64, page, size int) ([]model.TaskRun, int64, error) {
	var runs []model.TaskRun
	var total int64

	q := s.db.Model(&model.TaskRun{}).Where("task_id = ?", taskID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := q.Order("started_at DESC").Offset(offset).Limit(size).Find(&runs).Error; err != nil {
		return nil, 0, err
	}

	return runs, total, nil
}

// DeleteOlderThan removes task runs with started_at before the given cutoff time.
// Returns the number of deleted records.
func (s *TaskRunStore) DeleteOlderThan(cutoff time.Time) (int64, error) {
	result := s.db.Where("started_at < ?", cutoff).Delete(&model.TaskRun{})
	return result.RowsAffected, result.Error
}

// GetLatestByTask returns the most recent task run for a given task.
func (s *TaskRunStore) GetLatestByTask(taskID int64) (*model.TaskRun, error) {
	var run model.TaskRun
	if err := s.db.Where("task_id = ?", taskID).Order("started_at DESC").First(&run).Error; err != nil {
		return nil, err
	}
	return &run, nil
}
