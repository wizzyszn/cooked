package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleModerator Role = "moderator"
	RoleAdmin     Role = "admin"
)

// UserRole is an additive authorization grant. Every active account has the
// user role; staff accounts may additionally hold moderator or admin.
type UserRole struct {
	UserID    uuid.UUID  `gorm:"type:uuid;primaryKey" json:"-"`
	Role      Role       `gorm:"size:32;primaryKey" json:"role"`
	GrantedBy *uuid.UUID `gorm:"type:uuid" json:"granted_by,omitempty"`
	CreatedAt time.Time  `gorm:"not null;autoCreateTime" json:"created_at"`
}

func (UserRole) TableName() string { return "user_roles" }
