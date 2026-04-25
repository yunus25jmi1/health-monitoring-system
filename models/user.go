package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	RolePatient = "patient"
	RoleDoctor  = "doctor"
	RoleDevice  = "device"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Email     string    `gorm:"size:150;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	Role      string    `gorm:"size:20;not null" json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func IsValidRole(role string) bool {
	switch role {
	case RolePatient, RoleDoctor, RoleDevice:
		return true
	default:
		return false
	}
}

func (u *User) BeforeSave(tx *gorm.DB) error {
	u.Role = strings.ToLower(strings.TrimSpace(u.Role))
	if !IsValidRole(u.Role) {
		return errors.New("invalid role")
	}
	return nil
}
