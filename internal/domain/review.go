package domain

import (
	"time"

	"github.com/google/uuid"
)

type ReviewModerationStatus string

const (
	ReviewVisible ReviewModerationStatus = "visible"
	ReviewHidden  ReviewModerationStatus = "hidden"
	ReviewRemoved ReviewModerationStatus = "removed"
)

type Review struct {
	ID                 uuid.UUID              `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID             uuid.UUID              `gorm:"type:uuid;not null" json:"user_id"`
	RecipeID           uuid.UUID              `gorm:"type:uuid;not null" json:"recipe_id"`
	RecipeVersionID    uuid.UUID              `gorm:"type:uuid;not null" json:"recipe_version_id"`
	Taste              int                    `json:"taste"`
	Clarity            int                    `json:"clarity"`
	DifficultyAccuracy int                    `json:"difficulty_accuracy"`
	Comment            string                 `json:"comment,omitempty"`
	PhotoMediaID       *uuid.UUID             `gorm:"type:uuid" json:"photo_media_id,omitempty"`
	ModerationStatus   ReviewModerationStatus `json:"moderation_status"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

func (Review) TableName() string { return "reviews" }

type ReviewAggregate struct {
	RecipeVersionID          uuid.UUID `json:"recipe_version_id"`
	ReviewCount              int       `json:"review_count"`
	AverageTaste             float64   `json:"average_taste"`
	AverageClarity           float64   `json:"average_clarity"`
	AverageDifficultyAccuracy float64  `json:"average_difficulty_accuracy"`
	UpdatedAt                time.Time `json:"updated_at"`
}

func (ReviewAggregate) TableName() string { return "recipe_version_review_aggregates" }
