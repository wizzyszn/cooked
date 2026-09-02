package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type CookSessionStatus string

const (
	CookSessionInProgress CookSessionStatus = "in_progress"
	CookSessionCompleted  CookSessionStatus = "completed"
	CookSessionAbandoned  CookSessionStatus = "abandoned"
)

type CookSession struct {
	ID                  uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID              uuid.UUID         `gorm:"type:uuid" json:"user_id"`
	RecipeID            uuid.UUID         `gorm:"type:uuid" json:"recipe_id"`
	RecipeVersionID     uuid.UUID         `gorm:"type:uuid" json:"recipe_version_id"`
	Status              CookSessionStatus `json:"status"`
	PhotoMediaID        *uuid.UUID        `gorm:"type:uuid" json:"photo_media_id,omitempty"`
	StartedAt           time.Time         `json:"started_at"`
	LastActivityAt      time.Time         `json:"last_activity_at"`
	CompletedAt         *time.Time        `json:"completed_at,omitempty"`
	AbandonedAt         *time.Time        `json:"abandoned_at,omitempty"`
	CompletionLocalDate *time.Time        `gorm:"type:date" json:"completion_local_date,omitempty"`
	CompletionTimezone  *string           `json:"completion_timezone,omitempty"`
	XPAwarded           int               `json:"xp_awarded"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

func (CookSession) TableName() string { return "cook_sessions" }

type CookTimer struct {
	ID               uuid.UUID  `json:"id"`
	CookSessionID    uuid.UUID  `json:"cook_session_id"`
	RecipeStepID     uuid.UUID  `json:"recipe_step_id"`
	State            string     `json:"state"`
	DurationSeconds  int        `json:"duration_seconds"`
	RemainingSeconds int        `json:"remaining_seconds"`
	TargetAt         *time.Time `json:"target_at,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (CookTimer) TableName() string { return "cook_timers" }

type AnalyticsEvent struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID          *uuid.UUID
	AnonymousID     *uuid.UUID
	EventName       string
	SchemaVersion   int
	Source          string
	CookSessionID   *uuid.UUID
	RecipeID        *uuid.UUID
	RecipeVersionID *uuid.UUID
	IdempotencyKey  *string
	Properties      datatypes.JSON
	OccurredAt      time.Time
}

func (AnalyticsEvent) TableName() string { return "analytics_events" }
