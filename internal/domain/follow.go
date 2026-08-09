package domain

import (
	"time"

	"github.com/google/uuid"
)

// Follow is a directed social edge: FollowerID follows FollowingID.
type Follow struct {
	FollowerID  uuid.UUID `gorm:"type:uuid;primaryKey" json:"follower_id"`
	FollowingID uuid.UUID `gorm:"type:uuid;primaryKey;index" json:"following_id"`
	CreatedAt   time.Time `gorm:"not null;autoCreateTime" json:"created_at"`

	// Associations
	Follower  User `gorm:"foreignKey:FollowerID" json:"follower,omitempty"`
	Following User `gorm:"foreignKey:FollowingID" json:"following,omitempty"`
}

func (Follow) TableName() string { return "follows" }
