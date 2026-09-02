package domain

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
	"time"
)

type DelicacyStatus string

const (
	DelicacyPending   DelicacyStatus = "pending"
	DelicacyPublished DelicacyStatus = "published"
	DelicacyRejected  DelicacyStatus = "rejected"
	DelicacyWithdrawn DelicacyStatus = "withdrawn"
	DelicacyRetired   DelicacyStatus = "retired"
)

// Delicacy is a shared dish identity (e.g. "Jollof Rice").
// Multiple users can publish recipes for the same delicacy.
// CreatedBy is set when a user contributes the catalog entry; nil for system-seeded dishes.
type Delicacy struct {
	BaseModel
	Name             string         `gorm:"size:255;not null" json:"name"`
	Description      string         `gorm:"type:text;not null" json:"description"`
	ThumbnailURLs    []string       `gorm:"type:jsonb;serializer:json" json:"thumbnail_urls,omitempty"`
	CreatedBy        *uuid.UUID     `gorm:"type:uuid;index" json:"created_by,omitempty"`
	CategoryID       *uuid.UUID     `gorm:"type:uuid" json:"category_id,omitempty"`
	CoverMediaID     *uuid.UUID     `gorm:"type:uuid" json:"cover_media_id,omitempty"`
	Status           DelicacyStatus `json:"status"`
	CountryCodes     pq.StringArray `gorm:"type:text[]" json:"country_codes,omitempty"`
	OriginNotes      string         `json:"origin_notes,omitempty"`
	SubmittedAt      *time.Time     `json:"submitted_at,omitempty"`
	PublishedAt      *time.Time     `json:"published_at,omitempty"`
	ModeratedAt      *time.Time     `json:"-"`
	ModeratedBy      *uuid.UUID     `gorm:"type:uuid" json:"-"`
	ModerationReason string         `json:"-"`

	// Associations
	Creator  *User           `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Recipes  []Recipe        `gorm:"foreignKey:DelicacyID" json:"recipes,omitempty"`
	Tags     []Tag           `gorm:"many2many:delicacy_tags;" json:"tags,omitempty"`
	Aliases  []DelicacyAlias `gorm:"foreignKey:DelicacyID" json:"aliases,omitempty"`
	Regions  []Region        `gorm:"many2many:delicacy_regions" json:"regions,omitempty"`
	Category *Category       `json:"category,omitempty"`
}

func (Delicacy) TableName() string { return "delicacies" }

type DelicacyAlias struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	DelicacyID uuid.UUID `gorm:"type:uuid" json:"-"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"-"`
}
type Category struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	RetiredAt *time.Time `json:"retired_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
type Region struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	RetiredAt *time.Time `json:"retired_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
type MeasurementUnit struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name      string     `json:"name"`
	Symbol    string     `json:"symbol"`
	RetiredAt *time.Time `json:"retired_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
