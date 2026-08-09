package domain

import (
	"time"

	"github.com/google/uuid"
)

// Rating is one user's score (and optional review) for a recipe.
// At most one rating per (user, recipe).
type Rating struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_rating_user_recipe" json:"user_id"`
	RecipeID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_rating_user_recipe;index" json:"recipe_id"`
	Score     int16     `gorm:"not null" json:"score"` // 1–5; enforced in DB CHECK
	Body      string    `gorm:"type:text" json:"body,omitempty"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime" json:"updated_at"`

	// Associations
	User   User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Recipe Recipe `gorm:"foreignKey:RecipeID" json:"-"`
}

func (Rating) TableName() string { return "ratings" }
