package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errRefreshReuse    = errors.New("refresh token reuse")
	errRefreshNotFound = errors.New("refresh token not found")
	errRefreshExpired  = errors.New("refresh token expired")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) CreateRefreshToken(ctx context.Context, refreshToken *domain.RefreshToken) error {
	if err := r.db.WithContext(ctx).Create(refreshToken).Error; err != nil {
		return err
	}
	return nil
}

func (r *Repository) FindRefreshTokenByHash(ctx context.Context, refreshTokenHash string) (*domain.RefreshToken, error) {
	var t domain.RefreshToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", refreshTokenHash).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repository) Revoke(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("id = ? AND revoked_at IS NULL", id).Update("revoked_at", time.Now().UTC()).Error
}

func (r *Repository) RevokeFamily(ctx context.Context, familyID uuid.UUID) (int64, error) {
	res := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", familyID).Update("revoked_at", time.Now().UTC())
	return res.RowsAffected, res.Error
}

func (r *Repository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now().UTC())
	return res.RowsAffected, res.Error
}

func (r *Repository) MarkReplaced(ctx context.Context, oldID, newID uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("id = ? AND revoked_at IS NULL", oldID).Updates(map[string]interface{}{
		"replaced_by_id": newID,
		"revoked_at":     time.Now().UTC(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("token already revoked or missing")
	}
	return nil
}

// RotateRefreshToken locks the parent row, inserts next, and marks the parent
// replaced in one transaction so concurrent refreshes cannot mint two live children.
func (r *Repository) RotateRefreshToken(ctx context.Context, parentID uuid.UUID, next *domain.RefreshToken) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var parent domain.RefreshToken
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", parentID).
			First(&parent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errRefreshNotFound
		}
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if parent.RevokedAt != nil {
			if err := tx.Model(&domain.RefreshToken{}).
				Where("family_id = ? AND revoked_at IS NULL", parent.FamilyID).
				Update("revoked_at", now).Error; err != nil {
				return err
			}
			return errRefreshReuse
		}
		if parent.ExpiresAt.Before(now) {
			if err := tx.Model(&domain.RefreshToken{}).
				Where("id = ? AND revoked_at IS NULL", parent.ID).
				Update("revoked_at", now).Error; err != nil {
				return err
			}
			return errRefreshExpired
		}
		if err := tx.Create(next).Error; err != nil {
			return err
		}
		res := tx.Model(&domain.RefreshToken{}).
			Where("id = ? AND revoked_at IS NULL", parent.ID).
			Updates(map[string]interface{}{
				"replaced_by_id": next.ID,
				"revoked_at":     now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errRefreshReuse
		}
		return nil
	})
}

func (r *Repository) CreatePasswordResetToken(ctx context.Context, resetTokenPayload *domain.PasswordResetToken) error {
	return r.db.WithContext(ctx).Create(resetTokenPayload).Error
}

func (r *Repository) GetPasswordResetToken(ctx context.Context, otp string, userID uuid.UUID) (*domain.PasswordResetToken, error) {
	var t domain.PasswordResetToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND code = ? AND used_at IS NULL", userID, otp).
		First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *Repository) InvalidateUnusedPasswordResetTokens(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domain.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", now).Error
}

func (r *Repository) MarkPasswordResetTokenAsUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&domain.PasswordResetToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
