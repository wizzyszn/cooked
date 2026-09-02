package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type OAuthFlow struct {
	StateHash    string
	CodeVerifier string
	NonceHash    string
	ReturnURL    string
	ExpiresAt    time.Time
	UsedAt       *time.Time
	CreatedAt    time.Time
}

func (OAuthFlow) TableName() string { return "oauth_authorization_flows" }

type OAuthLoginCode struct {
	CodeHash  string
	UserID    uuid.UUID
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (OAuthLoginCode) TableName() string { return "oauth_login_codes" }

func (r *Repository) CreateOAuthFlow(ctx context.Context, flow *OAuthFlow) error {
	return r.db.WithContext(ctx).Create(flow).Error
}
func (r *Repository) ConsumeOAuthFlow(ctx context.Context, hash string, now time.Time) (*OAuthFlow, error) {
	var out OAuthFlow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&out, "state_hash = ?", hash).Error; err != nil {
			return err
		}
		if out.UsedAt != nil || !out.ExpiresAt.After(now) {
			return errOAuthCodeInvalid
		}
		return tx.Model(&OAuthFlow{}).Where("state_hash = ? AND used_at IS NULL", hash).Update("used_at", now).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errOAuthCodeInvalid
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (r *Repository) ResolveGoogleUser(ctx context.Context, subject, email, name, picture, username string, now time.Time) (*domain.User, error) {
	var account domain.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var identity domain.OAuthIdentity
		err := tx.Where("provider = 'google' AND provider_subject = ?", subject).First(&identity).Error
		if err == nil {
			return tx.First(&account, "id = ? AND deactivated_at IS NULL", identity.UserID).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		err = tx.Where("lower(email) = ? AND deactivated_at IS NULL", strings.ToLower(email)).First(&account).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			account = domain.User{Email: strings.ToLower(email), Name: name, UserName: username, Picture: picture, IsVerified: true, Timezone: "UTC"}
			if err = tx.Create(&account).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if !account.IsVerified {
			if err := tx.Model(&domain.User{}).Where("id = ?", account.ID).Update("is_verified", true).Error; err != nil {
				return err
			}
			account.IsVerified = true
		}
		return tx.Create(&domain.OAuthIdentity{UserID: account.ID, Provider: "google", ProviderSubject: subject, Email: strings.ToLower(email), CreatedAt: now}).Error
	})
	if err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Preload("Roles").Preload("DietaryPreferences").First(&account, "id = ?", account.ID).Error; err != nil {
		return nil, err
	}
	return &account, nil
}
func (r *Repository) CreateOAuthLoginCode(ctx context.Context, code *OAuthLoginCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}
func (r *Repository) ConsumeOAuthLoginCode(ctx context.Context, hash string, now time.Time) (*OAuthLoginCode, error) {
	var out OAuthLoginCode
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&out, "code_hash = ?", hash).Error; err != nil {
			return err
		}
		if out.UsedAt != nil || !out.ExpiresAt.After(now) {
			return errOAuthCodeInvalid
		}
		return tx.Model(&OAuthLoginCode{}).Where("code_hash = ? AND used_at IS NULL", hash).Update("used_at", now).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errOAuthCodeInvalid
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

var errOAuthCodeInvalid = errors.New("oauth code is invalid or expired")

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
