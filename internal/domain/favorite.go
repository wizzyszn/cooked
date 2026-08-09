package domain

import (
	"time"

	"github.com/google/uuid"
)

// Favorite is a user's saved recipe (bookmark).
type Favorite struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	RecipeID  uuid.UUID `gorm:"type:uuid;primaryKey;index" json:"recipe_id"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime" json:"created_at"`

	// Associations
	User   User   `gorm:"foreignKey:UserID" json:"-"`
	Recipe Recipe `gorm:"foreignKey:RecipeID" json:"recipe,omitempty"`
}

func (Favorite) TableName() string { return "favorites" }
