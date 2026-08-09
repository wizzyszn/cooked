package domain

import "github.com/google/uuid"

// RecipeIngredient links a recipe to a catalog ingredient with quantity/order.
// Quantity is nil when the amount is "to taste" or unspecified.
type RecipeIngredient struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RecipeID     uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_recipe_ingredient" json:"recipe_id"`
	IngredientID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_recipe_ingredient;index" json:"ingredient_id"`
	Quantity     *float64  `gorm:"type:numeric(12,3)" json:"quantity,omitempty"`
	Unit         *string   `gorm:"size:32" json:"unit,omitempty"`
	Note         string    `gorm:"size:255" json:"note,omitempty"`
	Position     int       `gorm:"not null;default:0" json:"position"`

	// Associations
	Recipe     Recipe     `gorm:"foreignKey:RecipeID" json:"-"`
	Ingredient Ingredient `gorm:"foreignKey:IngredientID" json:"ingredient,omitempty"`
}

func (RecipeIngredient) TableName() string { return "recipe_ingredients" }
