package domain

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"time"
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
	NotificationStatusStarted    NotificationStatus = "started"
)

type Notification struct {
	BaseModel
	UserID         uuid.UUID           `gorm:"type:uuid;not null;index" json:"user_id"`
	Channel        NotificationChannel `gorm:"size:16;not null;default:'email'" json:"channel"`
	Template       string              `gorm:"size:64;not null" json:"template"`
	Category       string              `gorm:"size:16;not null;default:transactional" json:"category"`
	PayloadJSON    datatypes.JSON      `gorm:"type:jsonb;column:payload_json" json:"payload"`
	Status         NotificationStatus  `gorm:"size:16;not null;default:'pending'" json:"status"`
	ExternalRef    *string             `gorm:"size:255" json:"external_ref,omitempty"`
	AttemptCount   int                 `json:"-"`
	NextAttemptAt  time.Time           `json:"-"`
	LockedAt       *time.Time          `json:"-"`
	LockedBy       *string             `json:"-"`
	LastError      *string             `json:"-"`
	SentAt         *time.Time          `json:"sent_at,omitempty"`
	ReadAt         *time.Time          `json:"read_at,omitempty"`
	IdempotencyKey *string             `gorm:"size:255" json:"-"`
}

type NotificationDeliveryAttempt struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	NotificationID uuid.UUID          `gorm:"type:uuid;not null;index"`
	AttemptNumber  int                `gorm:"not null"`
	ProviderKey    string             `gorm:"size:255;not null;uniqueIndex"`
	Status         NotificationStatus `gorm:"size:16;not null"`
	ExternalRef    *string            `gorm:"size:255"`
	Error          *string            `gorm:"type:text"`
	StartedAt      time.Time          `gorm:"not null;autoCreateTime"`
	CompletedAt    *time.Time
}

func (Notification) TableName() string                { return "notifications" }
func (NotificationDeliveryAttempt) TableName() string { return "notification_delivery_attempts" }
