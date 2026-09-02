package user

import (
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
)

type UpdateProfileRequest struct {
	Name          *string    `json:"name" binding:"omitempty,min=3,max=32"`
	UserName      *string    `json:"user_name" binding:"omitempty,alphanum,min=3,max=24"`
	Bio           *string    `json:"bio" binding:"omitempty,max=1024"`
	Timezone      *string    `json:"timezone" binding:"omitempty,max=64"`
	AvatarMediaID *uuid.UUID `json:"avatar_media_id"`
}

type DietaryPreferencesRequest struct {
	Slugs []string `json:"slugs" binding:"required,max=20,dive,min=1,max=64"`
}

type RoleChangeRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=1000"`
}

// PrivateProfile is the authenticated account projection returned by the API.
// It intentionally lives outside domain because it is a transport contract,
// not a persisted entity.
type PrivateProfile struct {
	ID                 string              `json:"id"`
	Email              string              `json:"email"`
	Name               string              `json:"name"`
	UserName           string              `json:"user_name"`
	IsVerified         bool                `json:"is_verified"`
	Picture            string              `json:"picture,omitempty"`
	Roles              []domain.Role       `json:"roles"`
	Bio                *string             `json:"bio,omitempty"`
	AvatarMediaID      *uuid.UUID          `json:"avatar_media_id,omitempty"`
	Timezone           string              `json:"timezone"`
	DietaryPreferences []domain.DietaryTag `json:"dietary_preferences"`
	XPTotal            int64               `json:"xp_total"`
	CurrentStreak      int                 `json:"current_streak"`
	LongestStreak      int                 `json:"longest_streak"`
}

type PublicProfile struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	UserName      string     `json:"user_name"`
	Picture       string     `json:"picture,omitempty"`
	Bio           *string    `json:"bio,omitempty"`
	AvatarMediaID *uuid.UUID `json:"avatar_media_id,omitempty"`
	XPTotal       int64      `json:"xp_total"`
	CurrentStreak int        `json:"current_streak"`
	LongestStreak int        `json:"longest_streak"`
}

func ToPrivateProfile(account *domain.User) *PrivateProfile {
	if account == nil {
		return nil
	}
	roles := make([]domain.Role, 0, len(account.Roles)+1)
	seen := make(map[domain.Role]struct{}, len(account.Roles)+1)
	for _, grant := range account.Roles {
		if _, exists := seen[grant.Role]; exists {
			continue
		}
		seen[grant.Role] = struct{}{}
		roles = append(roles, grant.Role)
	}
	if _, exists := seen[domain.RoleUser]; !exists {
		roles = append(roles, domain.RoleUser)
	}
	return &PrivateProfile{
		ID: account.ID.String(), Email: account.Email, Name: account.Name,
		UserName: account.UserName, IsVerified: account.IsVerified,
		Picture: account.Picture, Roles: roles, Bio: account.Bio,
		AvatarMediaID: account.AvatarMediaID, Timezone: account.Timezone,
		DietaryPreferences: account.DietaryPreferences, XPTotal: account.XPTotal,
		CurrentStreak: account.CurrentStreak, LongestStreak: account.LongestStreak,
	}
}
