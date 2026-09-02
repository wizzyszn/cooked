package review

import (
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
)

type WriteRequest struct {
	Taste              int        `json:"taste"`
	Clarity            int        `json:"clarity"`
	DifficultyAccuracy int        `json:"difficulty_accuracy"`
	Comment            string     `json:"comment"`
	PhotoMediaID       *uuid.UUID `json:"photo_media_id"`
}

type ReportRequest struct {
	TargetType string    `json:"target_type"`
	TargetID   uuid.UUID `json:"target_id"`
	Reason     string    `json:"reason"`
	Details    string    `json:"details"`
}

type ModerationRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type Report struct {
	ID         uuid.UUID  `json:"id"`
	ReporterID uuid.UUID  `json:"reporter_id"`
	TargetType string     `json:"target_type"`
	TargetID   uuid.UUID  `json:"target_id"`
	Reason     string     `json:"reason"`
	Details    string     `json:"details,omitempty"`
	State      string     `json:"state"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

type ReviewList struct {
	Items     []domain.Review        `json:"items"`
	Aggregate domain.ReviewAggregate `json:"aggregate"`
}
