package store

import (
	"github.com/pstrr/kronos/internal/model"
	"gorm.io/gorm"
)

// NotificationStore provides CRUD operations for notifications.
type NotificationStore struct {
	db *gorm.DB
}

// NewNotificationStore creates a new NotificationStore.
func NewNotificationStore(db *gorm.DB) *NotificationStore {
	return &NotificationStore{db: db}
}

// Create inserts a new notification.
func (s *NotificationStore) Create(n *model.Notification) error {
	return s.db.Create(n).Error
}

// GetByID finds a notification by primary key.
func (s *NotificationStore) GetByID(id int64) (*model.Notification, error) {
	var n model.Notification
	if err := s.db.First(&n, id).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// Update saves changes to an existing notification.
func (s *NotificationStore) Update(n *model.Notification) error {
	return s.db.Save(n).Error
}

// Delete removes a notification by ID.
func (s *NotificationStore) Delete(id int64) error {
	return s.db.Delete(&model.Notification{}, id).Error
}

// List returns a paginated list of all notifications.
func (s *NotificationStore) List(page, size int) ([]model.Notification, int64, error) {
	var items []model.Notification
	var total int64

	if err := s.db.Model(&model.Notification{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := s.db.Order("id DESC").Offset(offset).Limit(size).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ListByTask returns a paginated list of notifications for a specific task.
func (s *NotificationStore) ListByTask(taskID int64, page, size int) ([]model.Notification, int64, error) {
	var items []model.Notification
	var total int64

	q := s.db.Model(&model.Notification{}).Where("task_id = ?", taskID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := q.Order("id DESC").Offset(offset).Limit(size).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ListByTaskRun returns all notifications for a specific task run.
func (s *NotificationStore) ListByTaskRun(taskRunID int64) ([]model.Notification, error) {
	var items []model.Notification
	if err := s.db.Where("task_run_id = ?", taskRunID).Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
