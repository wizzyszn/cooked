package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel is shared by all persisted entities.
type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time      `gorm:"not null;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null;autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
