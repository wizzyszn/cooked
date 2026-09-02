package domain

import (
	"time"

	"github.com/google/uuid"
)

type OAuthIdentity struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID          uuid.UUID `gorm:"type:uuid;not null"`
	Provider        string    `gorm:"size:32;not null"`
	ProviderSubject string    `gorm:"size:255;not null"`
	Email           string    `gorm:"size:255;not null"`
	CreatedAt       time.Time
}

func (OAuthIdentity) TableName() string { return "oauth_identities" }
