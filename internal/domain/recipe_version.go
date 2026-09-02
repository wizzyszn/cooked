package domain

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"time"
)

type RecipeVersionLifecycle string

const (
	RecipeVersionDraft     RecipeVersionLifecycle = "draft"
	RecipeVersionPublished RecipeVersionLifecycle = "published"
)

type RecipeModerationStatus string

const (
	RecipeVisible RecipeModerationStatus = "visible"
	RecipeHidden  RecipeModerationStatus = "hidden"
	RecipeRemoved RecipeModerationStatus = "removed"
)

type RecipeVersion struct {
	ID              uuid.UUID                 `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RecipeID        uuid.UUID                 `gorm:"type:uuid" json:"recipe_id"`
	VersionNumber   int                       `json:"version_number"`
	Lifecycle       RecipeVersionLifecycle    `json:"lifecycle"`
	Title           string                    `json:"title"`
	Summary         string                    `json:"summary"`
	BaseServings    *int                      `json:"base_servings,omitempty"`
	PrepTimeSeconds *int                      `json:"prep_time_seconds,omitempty"`
	CookTimeSeconds *int                      `json:"cook_time_seconds,omitempty"`
	Difficulty      *RecipeDifficulty         `json:"difficulty,omitempty"`
	Notes           string                    `json:"notes,omitempty"`
	LegacyImageURLs datatypes.JSON            `gorm:"type:jsonb" json:"legacy_image_urls,omitempty"`
	PublishedAt     *time.Time                `json:"published_at,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
	Outdated        bool                      `gorm:"-" json:"outdated"`
	Ingredients     []RecipeVersionIngredient `gorm:"foreignKey:RecipeVersionID" json:"ingredients"`
	Steps           []RecipeVersionStep       `gorm:"foreignKey:RecipeVersionID" json:"steps"`
	Tags            []Tag                     `gorm:"many2many:recipe_version_tags" json:"tags,omitempty"`
	Media           []MediaAsset              `gorm:"many2many:recipe_version_media" json:"media,omitempty"`
}
type RecipeVersionIngredient struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RecipeVersionID   uuid.UUID      `gorm:"type:uuid" json:"-"`
	IngredientID      *uuid.UUID     `gorm:"type:uuid" json:"ingredient_id,omitempty"`
	Name              string         `json:"name"`
	Quantity          *float64       `json:"quantity,omitempty"`
	MeasurementUnitID *uuid.UUID     `gorm:"type:uuid" json:"measurement_unit_id,omitempty"`
	DisplayAmount     string         `json:"display_amount,omitempty"`
	SubstitutionNote  string         `json:"substitution_note,omitempty"`
	Position          int            `json:"position"`
	DeletedAt         gorm.DeletedAt `json:"-"`
	ScaledQuantity    *float64       `gorm:"-" json:"scaled_quantity,omitempty"`
	Scalable          bool           `gorm:"-" json:"scalable"`
}
type RecipeVersionStep struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RecipeVersionID uuid.UUID      `gorm:"type:uuid" json:"-"`
	Position        int            `json:"position"`
	Title           string         `json:"title"`
	Instruction     string         `json:"instruction"`
	Action          string         `json:"action"`
	DurationSeconds *int           `json:"duration_seconds,omitempty"`
	TechniqueTags   pq.StringArray `gorm:"type:text[]" json:"technique_tags,omitempty"`
	DeletedAt       gorm.DeletedAt `json:"-"`
}
