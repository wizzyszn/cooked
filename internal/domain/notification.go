package domain

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type NotificationChannel string
type NotificationStatus string

const (
	NotificationChannelEmail NotificationChannel = "email"
	NotificationChannelInApp NotificationChannel = "in_app"
)

const (
	NotificationStatusSent       NotificationStatus = "sent"
	NotificationStatusSuppressed NotificationStatus = "suppressed"
	NotificationStatusPending    NotificationStatus = "pending"
	NotificationStatusFailed     NotificationStatus = "failed"
)

type Notification struct {
	BaseModel
	UserID      uuid.UUID           `gorm:"type:uuid;not null;index" json:"user_id"`
	Channel     NotificationChannel `gorm:"size:16;not null;default:'email'" json:"channel"`
	Template    string              `gorm:"size:64;not null" json:"template"`
	PayloadJSON datatypes.JSON      `gorm:"type:jsonb;column:payload_json" json:"payload"`
	Status      NotificationStatus  `gorm:"size:16;not null;default:'pending'" json:"status"`
	ExternalRef *string             `gorm:"size:255" json:"external_ref,omitempty"`
}

func (Notification) TableName() string { return "notifications" }
