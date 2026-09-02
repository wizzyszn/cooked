package domain

import (
	"time"

	"github.com/google/uuid"
)

type DietaryTag struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Slug      string    `gorm:"size:64;not null;uniqueIndex" json:"slug"`
	Active    bool      `gorm:"not null;default:true" json:"-"`
	CreatedAt time.Time `json:"-"`
}

func (DietaryTag) TableName() string { return "dietary_tags" }
