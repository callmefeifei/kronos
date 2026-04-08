package model

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a Kronos user account.
type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"type:varchar(255);not null" json:"-"`
	DisplayName  string    `gorm:"type:varchar(128)" json:"display_name"`
	Role         string    `gorm:"type:varchar(16);not null;default:user" json:"role"`   // admin, user
	Status       string    `gorm:"type:varchar(16);not null;default:active" json:"status"` // active, disabled
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string { return "users" }

// SetPassword hashes the given plaintext password using bcrypt.
func (u *User) SetPassword(plain string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword compares the given plaintext password against the stored hash.
func (u *User) CheckPassword(plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plain)) == nil
}
