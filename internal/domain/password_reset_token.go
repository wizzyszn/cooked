package domain

import (
	"time"

	"github.com/google/uuid"
)

type PasswordResetToken struct {
	BaseModel
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Code      string    `gorm:"size:64;not null;index"`
	ExpiresAt time.Time `gorm:"not null;index"`
	UsedAt    *time.Time
}

func (PasswordResetToken) TableName() string { return "password_reset_tokens" }
