package recipe

import (
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
)

type CreateRequest struct {
	DelicacyID uuid.UUID               `json:"delicacy_id" binding:"required"`
	Visibility domain.RecipeVisibility `json:"visibility"`
	Snapshot   Snapshot                `json:"snapshot" binding:"required"`
}
type Snapshot struct {
	Title           string                   `json:"title"`
	Summary         string                   `json:"summary"`
	BaseServings    *int                     `json:"base_servings"`
	PrepTimeSeconds *int                     `json:"prep_time_seconds"`
	CookTimeSeconds *int                     `json:"cook_time_seconds"`
	Difficulty      *domain.RecipeDifficulty `json:"difficulty"`
	Notes           string                   `json:"notes"`
	Ingredients     []IngredientInput        `json:"ingredients"`
	Steps           []StepInput              `json:"steps"`
	TagIDs          []uuid.UUID              `json:"tag_ids"`
	CoverMediaIDs   []uuid.UUID              `json:"cover_media_ids"`
}
type IngredientInput struct {
	ID                uuid.UUID  `json:"id"`
	IngredientID      *uuid.UUID `json:"ingredient_id"`
	Name              string     `json:"name"`
	Quantity          *float64   `json:"quantity"`
	MeasurementUnitID *uuid.UUID `json:"measurement_unit_id"`
	DisplayAmount     string     `json:"display_amount"`
	SubstitutionNote  string     `json:"substitution_note"`
	Position          int        `json:"position"`
}
type StepInput struct {
	ID                 uuid.UUID   `json:"id"`
	Position           int         `json:"position"`
	Title              string      `json:"title"`
	Instruction        string      `json:"instruction"`
	Action             string      `json:"action"`
	DurationSeconds    *int        `json:"duration_seconds"`
	TechniqueTags      []string    `json:"technique_tags"`
	IngredientEntryIDs []uuid.UUID `json:"ingredient_entry_ids"`
	MediaIDs           []uuid.UUID `json:"media_ids"`
}
type VisibilityRequest struct {
	Visibility domain.RecipeVisibility `json:"visibility" binding:"required"`
}
