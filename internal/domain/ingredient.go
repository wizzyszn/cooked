package domain

// Ingredient is a global catalog entry shared across recipes
// (e.g. "chicken", "tomato", "basmati rice").
type Ingredient struct {
	BaseModel
	Name        string  `gorm:"size:255;not null" json:"name"`
	DefaultUnit *string `gorm:"size:32" json:"default_unit,omitempty"`

	// Associations
	RecipeIngredients []RecipeIngredient `gorm:"foreignKey:IngredientID" json:"-"`
}

func (Ingredient) TableName() string { return "ingredients" }
