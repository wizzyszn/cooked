package domain

import "github.com/google/uuid"

// RecipeStep is one ordered instruction in a recipe.
// Algo on Recipe remains a free-text fallback / full-text blob.
type RecipeStep struct {
	BaseModel
	RecipeID        uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_recipe_step_pos" json:"recipe_id"`
	Position        int       `gorm:"not null;uniqueIndex:idx_recipe_step_pos" json:"position"`
	Body            string    `gorm:"type:text;not null" json:"body"`
	DurationMinutes *int      `json:"duration_minutes,omitempty"`
	ImageURL        string    `gorm:"size:512" json:"image_url,omitempty"`

	// Associations
	Recipe Recipe `gorm:"foreignKey:RecipeID" json:"-"`
}

func (RecipeStep) TableName() string { return "recipe_steps" }
