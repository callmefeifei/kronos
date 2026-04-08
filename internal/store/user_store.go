package store

import (
	"github.com/pstrr/kronos/internal/model"
	"gorm.io/gorm"
)

// UserStore provides CRUD operations for users.
type UserStore struct {
	db *gorm.DB
}

// NewUserStore creates a new UserStore.
func NewUserStore(db *gorm.DB) *UserStore {
	return &UserStore{db: db}
}

// Create inserts a new user.
func (s *UserStore) Create(user *model.User) error {
	return s.db.Create(user).Error
}

// GetByID finds a user by primary key.
func (s *UserStore) GetByID(id int64) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername finds a user by username.
func (s *UserStore) GetByUsername(username string) (*model.User, error) {
	var user model.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Update saves changes to an existing user.
func (s *UserStore) Update(user *model.User) error {
	return s.db.Save(user).Error
}

// Delete removes a user by ID (hard delete).
func (s *UserStore) Delete(id int64) error {
	return s.db.Delete(&model.User{}, id).Error
}

// List returns a paginated list of users.
func (s *UserStore) List(page, size int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	if err := s.db.Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := s.db.Order("id ASC").Offset(offset).Limit(size).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
