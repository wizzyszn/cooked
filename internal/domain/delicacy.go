package domain

import "github.com/google/uuid"

// Delicacy is a shared dish identity (e.g. "Jollof Rice").
// Multiple users can publish recipes for the same delicacy.
// CreatedBy is set when a user contributes the catalog entry; nil for system-seeded dishes.
type Delicacy struct {
	BaseModel
	Name          string     `gorm:"size:255;not null" json:"name"`
	Description   string     `gorm:"type:text;not null" json:"description"`
	ThumbnailURLs []string   `gorm:"type:jsonb;serializer:json" json:"thumbnail_urls,omitempty"`
	CreatedBy     *uuid.UUID `gorm:"type:uuid;index" json:"created_by,omitempty"`

	// Associations
	Creator *User    `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Recipes []Recipe `gorm:"foreignKey:DelicacyID" json:"recipes,omitempty"`
	Tags    []Tag    `gorm:"many2many:delicacy_tags;" json:"tags,omitempty"`
}

func (Delicacy) TableName() string { return "delicacies" }
