package user

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
)

func TestUpdateProfileRejectsInvalidIANATimezone(t *testing.T) {
	tz := "West-Africa/Lagos"
	_, err := NewService(&Repository{}, nil).UpdateProfile(t.Context(), uuid.New(), UpdateProfileRequest{Timezone: &tz})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrValidation.Code {
		t.Fatalf("error = %v, want validation error", err)
	}
}

func TestDietaryNoneMustBeEmptySelection(t *testing.T) {
	_, err := NewService(&Repository{}, nil).ReplaceDietary(t.Context(), uuid.New(), DietaryPreferencesRequest{Slugs: []string{"none"}})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrValidation.Code {
		t.Fatalf("error = %v, want validation error", err)
	}
}
