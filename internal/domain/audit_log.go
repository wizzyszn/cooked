package domain

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ActorID    *uuid.UUID `gorm:"type:uuid"`
	Action     string     `gorm:"size:128;not null"`
	TargetType string     `gorm:"size:64;not null"`
	TargetID   *uuid.UUID `gorm:"type:uuid"`
	Reason     string
	BeforeJSON []byte `gorm:"type:jsonb"`
	AfterJSON  []byte `gorm:"type:jsonb"`
	CreatedAt  time.Time
}
