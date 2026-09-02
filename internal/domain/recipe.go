package domain

import "github.com/google/uuid"

// RecipeVisibility controls who can discover a recipe.
type RecipeVisibility string

const (
	RecipeVisibilityPublic   RecipeVisibility = "public"
	RecipeVisibilityPrivate  RecipeVisibility = "private"
	RecipeVisibilityUnlisted RecipeVisibility = "unlisted"
)

// RecipeDifficulty is an optional cooking difficulty hint.
type RecipeDifficulty string

const (
	RecipeDifficultyEasy   RecipeDifficulty = "easy"
	RecipeDifficultyMedium RecipeDifficulty = "medium"
	RecipeDifficultyHard   RecipeDifficulty = "hard"
)

// Recipe is one user's method for making a delicacy.
// A user may publish multiple recipes for the same delicacy.
type Recipe struct {
	BaseModel
	UserID                    uuid.UUID              `gorm:"type:uuid;not null;index" json:"user_id"`
	DelicacyID                uuid.UUID              `gorm:"type:uuid;not null;index" json:"delicacy_id"`
	CurrentPublishedVersionID *uuid.UUID             `gorm:"type:uuid" json:"current_published_version_id,omitempty"`
	ModerationStatus          RecipeModerationStatus `json:"moderation_status"`

	Title   string `gorm:"size:255;not null" json:"title"`
	Summary string `gorm:"size:512" json:"summary,omitempty"`
	// Algo is free-text instructions; structured steps live in RecipeStep.
	Algo string   `gorm:"type:text;not null" json:"algo"`
	Imgs []string `gorm:"type:jsonb;serializer:json" json:"imgs,omitempty"`

	Visibility      RecipeVisibility  `gorm:"size:32;not null;default:'public';index" json:"visibility"`
	PrepTimeMinutes *int              `json:"prep_time_minutes,omitempty"`
	CookTimeMinutes *int              `json:"cook_time_minutes,omitempty"`
	Servings        *int              `json:"servings,omitempty"`
	Difficulty      *RecipeDifficulty `gorm:"size:32" json:"difficulty,omitempty"`

	// Denormalized rating stats (maintained by app/triggers when ratings change).
	AvgRating   float64 `gorm:"type:numeric(3,2);not null;default:0" json:"avg_rating"`
	RatingCount int     `gorm:"not null;default:0" json:"rating_count"`

	// Core associations
	User     User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Delicacy Delicacy `gorm:"foreignKey:DelicacyID" json:"delicacy,omitempty"`

	// Structured cooking data
	Ingredients []RecipeIngredient `gorm:"foreignKey:RecipeID" json:"ingredients,omitempty"`
	Steps       []RecipeStep       `gorm:"foreignKey:RecipeID" json:"steps,omitempty"`

	// Discovery + social
	Tags      []Tag      `gorm:"many2many:recipe_tags;" json:"tags,omitempty"`
	Favorites []Favorite `gorm:"foreignKey:RecipeID" json:"favorites,omitempty"`
	Ratings   []Rating   `gorm:"foreignKey:RecipeID" json:"ratings,omitempty"`
	Comments  []Comment  `gorm:"foreignKey:RecipeID" json:"comments,omitempty"`
}

func (Recipe) TableName() string { return "recipes" }
