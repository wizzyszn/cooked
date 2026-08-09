package domain

import "github.com/google/uuid"

// TagKind classifies tags for filtering (cuisine, diet, occasion, general).
type TagKind string

const (
	TagKindCuisine  TagKind = "cuisine"
	TagKindDiet     TagKind = "diet"
	TagKindOccasion TagKind = "occasion"
	TagKindGeneral  TagKind = "general"
)

// Tag is a discovery label applied to recipes and/or delicacies.
type Tag struct {
	BaseModel
	Name string  `gorm:"size:64;not null" json:"name"`
	Slug string  `gorm:"size:64;not null" json:"slug"`
	Kind TagKind `gorm:"size:32;not null;default:'general';index" json:"kind"`

	// Associations
	Recipes    []Recipe   `gorm:"many2many:recipe_tags;" json:"recipes,omitempty"`
	Delicacies []Delicacy `gorm:"many2many:delicacy_tags;" json:"delicacies,omitempty"`
}

func (Tag) TableName() string { return "tags" }

// RecipeTag is the recipes ↔ tags join row.
type RecipeTag struct {
	RecipeID uuid.UUID `gorm:"type:uuid;primaryKey" json:"recipe_id"`
	TagID    uuid.UUID `gorm:"type:uuid;primaryKey;index" json:"tag_id"`
}

func (RecipeTag) TableName() string { return "recipe_tags" }

// DelicacyTag is the delicacies ↔ tags join row.
type DelicacyTag struct {
	DelicacyID uuid.UUID `gorm:"type:uuid;primaryKey" json:"delicacy_id"`
	TagID      uuid.UUID `gorm:"type:uuid;primaryKey;index" json:"tag_id"`
}

func (DelicacyTag) TableName() string { return "delicacy_tags" }
