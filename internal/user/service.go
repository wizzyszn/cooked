package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
	log  *zap.SugaredLogger
}

func NewService(repo *Repository, log *zap.SugaredLogger) *Service {
	return &Service{repo: repo, log: log}
}

func (s *Service) Me(ctx context.Context, id uuid.UUID) (*PrivateProfile, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal(s.log, "get current user", err)
	}
	if u == nil || u.DeactivatedAt != nil {
		return nil, apperrors.ErrUnauthorized
	}
	return ToPrivateProfile(u), nil
}

func (s *Service) PublicProfile(ctx context.Context, username string) (*PublicProfile, error) {
	u, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, apperrors.Internal(s.log, "get public profile", err)
	}
	if u == nil {
		return nil, apperrors.ErrNotFound
	}
	if u.DeactivatedAt != nil {
		return &PublicProfile{ID: u.ID.String(), Name: "Deleted user", UserName: u.UserName}, nil
	}
	return &PublicProfile{ID: u.ID.String(), Name: u.Name, UserName: u.UserName, Picture: u.Picture, Bio: u.Bio, AvatarMediaID: u.AvatarMediaID, XPTotal: u.XPTotal, CurrentStreak: u.CurrentStreak, LongestStreak: u.LongestStreak}, nil
}

func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, req UpdateProfileRequest) (*PrivateProfile, error) {
	values := map[string]any{}
	if req.Name != nil {
		values["name"] = strings.TrimSpace(*req.Name)
	}
	if req.UserName != nil {
		name := strings.TrimSpace(*req.UserName)
		existing, err := s.repo.FindByUsername(ctx, name)
		if err != nil {
			return nil, apperrors.Internal(s.log, "profile username lookup", err)
		}
		if existing != nil && existing.ID != id {
			return nil, apperrors.ErrUsernameTaken
		}
		values["user_name"] = name
	}
	if req.Bio != nil {
		bio := strings.TrimSpace(*req.Bio)
		if bio == "" {
			values["bio"] = nil
		} else {
			values["bio"] = bio
		}
	}
	if req.Timezone != nil {
		tz := strings.TrimSpace(*req.Timezone)
		if _, err := time.LoadLocation(tz); err != nil {
			return nil, apperrors.WithMessage(apperrors.ErrValidation, "timezone must be a valid IANA timezone")
		}
		values["timezone"] = tz
	}
	if req.AvatarMediaID != nil {
		values["avatar_media_id"] = *req.AvatarMediaID
	}
	if len(values) == 0 {
		return s.Me(ctx, id)
	}
	if err := s.repo.UpdateProfile(ctx, id, values); err != nil {
		if IsUniqueViolation(err) {
			return nil, apperrors.ErrUsernameTaken
		}
		return nil, apperrors.Internal(s.log, "update profile", err)
	}
	return s.Me(ctx, id)
}

func (s *Service) ReplaceDietary(ctx context.Context, id uuid.UUID, req DietaryPreferencesRequest) (*PrivateProfile, error) {
	seen := map[string]bool{}
	slugs := make([]string, 0, len(req.Slugs))
	for _, raw := range req.Slugs {
		slug := strings.ToLower(strings.TrimSpace(raw))
		if slug == "none" {
			return nil, apperrors.WithMessage(apperrors.ErrValidation, "none must be represented by an empty selection")
		}
		if !seen[slug] {
			seen[slug] = true
			slugs = append(slugs, slug)
		}
	}
	if err := s.repo.ReplaceDietaryPreferences(ctx, id, slugs); err != nil {
		if errors.Is(err, ErrUnknownDietaryTag) {
			return nil, apperrors.WithMessage(apperrors.ErrValidation, "one or more dietary tags are invalid")
		}
		return nil, apperrors.Internal(s.log, "replace dietary preferences", err)
	}
	return s.Me(ctx, id)
}

func (s *Service) Anonymize(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Anonymize(ctx, id, time.Now().UTC()); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return apperrors.Internal(s.log, "anonymize account", err)
	}
	return nil
}
func (s *Service) SetRole(ctx context.Context, actor, target uuid.UUID, role domain.Role, grant bool, reason string) error {
	err := s.repo.SetRole(ctx, actor, target, role, grant, strings.TrimSpace(reason))
	if errors.Is(err, ErrInvalidRole) {
		return apperrors.ErrValidation
	}
	if errors.Is(err, ErrLastAdmin) {
		return apperrors.WithMessage(apperrors.ErrConflict, "the last admin role cannot be revoked")
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return apperrors.Internal(s.log, "change role", err)
	}
	return nil
}
