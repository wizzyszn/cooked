package domain

import (
	"time"

	"github.com/google/uuid"
)

// User is an account that authors recipes and may contribute delicacies.
type User struct {
	BaseModel
	Email              string       `gorm:"size:255;not null;uniqueIndex" json:"email"`
	Name               string       `gorm:"size:255;not null" json:"name"`
	UserName           string       `gorm:"size:24;not null" json:"user_name"`
	Picture            string       `gorm:"size:512" json:"picture,omitempty"`
	IsVerified         bool         `gorm:"not null;default:false" json:"is_verified"`
	HashPass           string       `gorm:"size:255" json:"-"`
	Bio                *string      `gorm:"size:1024" json:"bio,omitempty"`
	AvatarMediaID      *uuid.UUID   `gorm:"type:uuid" json:"avatar_media_id,omitempty"`
	Timezone           string       `gorm:"size:64;not null;default:UTC" json:"timezone"`
	AnonymizedAt       *time.Time   `json:"-"`
	DeactivatedAt      *time.Time   `json:"-"`
	XPTotal            int64        `gorm:"not null;default:0" json:"xp_total"`
	CurrentStreak      int          `gorm:"not null;default:0" json:"current_streak"`
	LongestStreak      int          `gorm:"not null;default:0" json:"longest_streak"`
	DietaryPreferences []DietaryTag `gorm:"many2many:user_dietary_preferences" json:"dietary_preferences,omitempty"`
	Roles              []UserRole   `gorm:"foreignKey:UserID" json:"roles,omitempty"`
	// Content
	Recipes    []Recipe   `gorm:"foreignKey:UserID" json:"recipes,omitempty"`
	Delicacies []Delicacy `gorm:"foreignKey:CreatedBy" json:"delicacies,omitempty"`

	// Social
	Favorites []Favorite `gorm:"foreignKey:UserID" json:"favorites,omitempty"`
	Ratings   []Rating   `gorm:"foreignKey:UserID" json:"ratings,omitempty"`
	Comments  []Comment  `gorm:"foreignKey:UserID" json:"comments,omitempty"`
	Following []Follow   `gorm:"foreignKey:FollowerID" json:"following,omitempty"`
	Followers []Follow   `gorm:"foreignKey:FollowingID" json:"followers,omitempty"`
}

func (User) TableName() string { return "users" }

func (u *User) HasRole(role Role) bool {
	if u == nil {
		return false
	}
	if role == RoleUser {
		return true
	}
	for _, grant := range u.Roles {
		if grant.Role == role {
			return true
		}
	}
	return false
}
