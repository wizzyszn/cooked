package cook

import (
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"time"
)

type StartRequest struct {
	RecipeVersionID uuid.UUID `json:"recipe_version_id" binding:"required"`
}
type TimerRequest struct {
	Action          string `json:"action" binding:"required"`
	DurationSeconds *int   `json:"duration_seconds"`
}
type CompleteRequest struct {
	PhotoMediaID *uuid.UUID `json:"photo_media_id"`
}
type EventRequest struct {
	AnonymousID     *uuid.UUID     `json:"anonymous_id"`
	EventName       string         `json:"event_name" binding:"required"`
	RecipeID        *uuid.UUID     `json:"recipe_id"`
	RecipeVersionID *uuid.UUID     `json:"recipe_version_id"`
	Properties      map[string]any `json:"properties"`
}
type SessionView struct {
	domain.CookSession
	VisitedStepIDs []uuid.UUID        `json:"visited_step_ids"`
	Timers         []domain.CookTimer `json:"timers"`
}
type SessionPage struct {
	Items      []SessionView `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty"`
}
type Metrics struct {
	ActivationCount           int64     `json:"activation_count"`
	CookModeEntries           int64     `json:"cook_mode_entries"`
	CompletedSessions         int64     `json:"completed_sessions"`
	CookModeConversion        float64   `json:"cook_mode_conversion"`
	ReviewEligibleCompletions int64     `json:"review_eligible_completions"`
	ActivatedCohortsMatured   int64     `json:"activated_cohorts_matured"`
	SevenDayReturners         int64     `json:"seven_day_returners"`
	SevenDayRetention         float64   `json:"seven_day_retention"`
	GeneratedAt               time.Time `json:"generated_at"`
}
