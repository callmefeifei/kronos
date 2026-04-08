package store

import (
	"github.com/pstrr/kronos/internal/model"
	"gorm.io/gorm"
)

// TaskFilter holds optional filters for task listing.
type TaskFilter struct {
	Type         string // remind, script, agent
	ScheduleType string // cron, interval, once
	Enabled      *bool
	Status       string // last_status filter
}

// TaskStore provides CRUD operations for tasks.
type TaskStore struct {
	db *gorm.DB
}

// NewTaskStore creates a new TaskStore.
func NewTaskStore(db *gorm.DB) *TaskStore {
	return &TaskStore{db: db}
}

// Create inserts a new task.
func (s *TaskStore) Create(task *model.Task) error {
	return s.db.Create(task).Error
}

// GetByID finds a task by primary key (respects soft delete).
func (s *TaskStore) GetByID(id int64) (*model.Task, error) {
	var task model.Task
	if err := s.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// Update saves changes to an existing task.
func (s *TaskStore) Update(task *model.Task) error {
	return s.db.Save(task).Error
}

// Delete soft-deletes a task by ID.
func (s *TaskStore) Delete(id int64) error {
	return s.db.Delete(&model.Task{}, id).Error
}

// List returns a paginated list of tasks with optional filters.
func (s *TaskStore) List(page, size int, filter *TaskFilter) ([]model.Task, int64, error) {
	var tasks []model.Task
	var total int64

	q := s.db.Model(&model.Task{})
	q = applyTaskFilter(q, filter)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := q.Order("id DESC").Offset(offset).Limit(size).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// ListByUser returns a paginated list of tasks owned by the given user.
func (s *TaskStore) ListByUser(userID int64, page, size int, filter *TaskFilter) ([]model.Task, int64, error) {
	var tasks []model.Task
	var total int64

	q := s.db.Model(&model.Task{}).Where("user_id = ?", userID)
	q = applyTaskFilter(q, filter)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := q.Order("id DESC").Offset(offset).Limit(size).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// ListEnabled returns all enabled tasks (for the scheduler).
func (s *TaskStore) ListEnabled() ([]model.Task, error) {
	var tasks []model.Task
	if err := s.db.Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// applyTaskFilter adds WHERE clauses based on the filter.
func applyTaskFilter(q *gorm.DB, filter *TaskFilter) *gorm.DB {
	if filter == nil {
		return q
	}
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.ScheduleType != "" {
		q = q.Where("schedule_type = ?", filter.ScheduleType)
	}
	if filter.Enabled != nil {
		q = q.Where("enabled = ?", *filter.Enabled)
	}
	if filter.Status != "" {
		q = q.Where("last_status = ?", filter.Status)
	}
	return q
}
