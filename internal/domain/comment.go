package domain

import "github.com/google/uuid"

// Comment is a threaded remark on a recipe.
// ParentID is set for replies; nil for top-level comments.
type Comment struct {
	BaseModel
	RecipeID uuid.UUID  `gorm:"type:uuid;not null;index" json:"recipe_id"`
	UserID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	ParentID *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	Body     string     `gorm:"type:text;not null" json:"body"`

	// Associations
	Recipe  Recipe    `gorm:"foreignKey:RecipeID" json:"-"`
	User    User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Parent  *Comment  `gorm:"foreignKey:ParentID" json:"-"`
	Replies []Comment `gorm:"foreignKey:ParentID" json:"replies,omitempty"`
}

func (Comment) TableName() string { return "comments" }
