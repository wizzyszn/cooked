package domain

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash    string     `gorm:"type:text;not null;uniqueIndex" json:"-"`
	FamilyID     uuid.UUID  `gorm:"type:uuid;not null" json:"family_id"`
	ParentID     *uuid.UUID `gorm:"type:uuid" json:"parent_id,omitempty"`
	ReplacedByID *uuid.UUID `gorm:"type:uuid" json:"replaced_by_id,omitempty"`
	ExpiresAt    time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	CreatedAt    time.Time  `gorm:"not null" json:"created_at"`
	UserAgent    string     `gorm:"size:512" json:"user_agent,omitempty"`
	IP           string     `gorm:"size:64" json:"ip,omitempty"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }
