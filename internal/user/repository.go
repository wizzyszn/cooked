package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wizzyszn/cooked/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository is the persistence boundary for user accounts.
// Auth and notify depend on this; they do not own user rows.
type Repository struct {
	db *gorm.DB
}

func NewRepository(database *gorm.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Preload("Roles").Preload("DietaryPreferences", "active = ?", true).
		Where("lower(email) = ?", strings.ToLower(strings.TrimSpace(email))).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Preload("Roles").Preload("DietaryPreferences", "active = ?", true).
		Where("lower(user_name) = ?", strings.ToLower(strings.TrimSpace(username))).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Preload("Roles").Preload("DietaryPreferences", "active = ?", true).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, id uuid.UUID, values map[string]any) error {
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ? AND deactivated_at IS NULL", id).Updates(values).Error
}

func (r *Repository) ReplaceDietaryPreferences(ctx context.Context, userID uuid.UUID, slugs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM user_dietary_preferences WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		if len(slugs) == 0 {
			return nil
		}
		var tags []domain.DietaryTag
		if err := tx.Where("active = true AND slug IN ?", slugs).Find(&tags).Error; err != nil {
			return err
		}
		if len(tags) != len(slugs) {
			return ErrUnknownDietaryTag
		}
		for _, tag := range tags {
			if err := tx.Exec("INSERT INTO user_dietary_preferences (user_id, dietary_tag_id) VALUES (?, ?)", userID, tag.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

var ErrUnknownDietaryTag = errors.New("unknown dietary tag")

func (r *Repository) Anonymize(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var account domain.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, "id = ? AND deactivated_at IS NULL", userID).Error; err != nil {
			return err
		}
		suffix := strings.ReplaceAll(userID.String(), "-", "")
		if err := tx.Model(&domain.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", userID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM oauth_identities WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM user_dietary_preferences WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE media_assets SET owner_id = NULL, processing_status = 'deleted', deleted_at = ?, updated_at = ? WHERE owner_id = ? AND (access_scope = 'private' OR purpose = 'profile_avatar')", now, now, userID).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE media_assets SET owner_id = NULL, updated_at = ? WHERE owner_id = ?", now, userID).Error; err != nil {
			return err
		}
		values := map[string]interface{}{"email": "deleted+" + suffix + "@deleted.invalid", "user_name": "deleted_" + suffix[:16], "name": "Deleted user", "picture": nil, "bio": nil, "avatar_media_id": nil, "hash_pass": "", "is_verified": false, "anonymized_at": now, "deactivated_at": now, "updated_at": now}
		if err := tx.Model(&domain.User{}).Where("id = ?", userID).Updates(values).Error; err != nil {
			return err
		}
		payload := []byte(`{"state":"anonymized"}`)
		return tx.Create(&domain.AuditLog{Action: "user.anonymized", TargetType: "user", TargetID: &userID, AfterJSON: payload, CreatedAt: now}).Error
	})
}

func (r *Repository) SetRole(ctx context.Context, actorID, targetID uuid.UUID, role domain.Role, grant bool, reason string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if role != domain.RoleModerator && role != domain.RoleAdmin {
			return ErrInvalidRole
		}
		var target domain.User
		if err := tx.First(&target, "id = ? AND deactivated_at IS NULL", targetID).Error; err != nil {
			return err
		}
		before := fmt.Appendf(nil, `{"role":%q,"granted":%t}`, role, hasRole(tx, targetID, role))
		if grant {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&domain.UserRole{UserID: targetID, Role: role, GrantedBy: &actorID}).Error; err != nil {
				return err
			}
		} else {
			if role == domain.RoleAdmin {
				var count int64
				if err := tx.Model(&domain.UserRole{}).Where("role = ?", domain.RoleAdmin).Count(&count).Error; err != nil {
					return err
				}
				if count <= 1 {
					return ErrLastAdmin
				}
			}
			if err := tx.Delete(&domain.UserRole{}, "user_id = ? AND role = ?", targetID, role).Error; err != nil {
				return err
			}
		}
		after := fmt.Appendf(nil, `{"role":%q,"granted":%t}`, role, grant)
		return tx.Create(&domain.AuditLog{ActorID: &actorID, Action: "user.role_changed", TargetType: "user", TargetID: &targetID, Reason: reason, BeforeJSON: before, AfterJSON: after}).Error
	})
}

func (r *Repository) BootstrapAdmin(ctx context.Context, email, reason string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target domain.User
		if err := tx.Where("lower(email) = ? AND deactivated_at IS NULL", strings.ToLower(strings.TrimSpace(email))).First(&target).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&domain.UserRole{UserID: target.ID, Role: domain.RoleAdmin}).Error; err != nil {
			return err
		}
		return tx.Create(&domain.AuditLog{Action: "user.admin_bootstrapped", TargetType: "user", TargetID: &target.ID, Reason: reason, AfterJSON: []byte(`{"role":"admin","granted":true}`)}).Error
	})
}

func hasRole(tx *gorm.DB, id uuid.UUID, role domain.Role) bool {
	var count int64
	return tx.Model(&domain.UserRole{}).Where("user_id = ? AND role = ?", id, role).Count(&count).Error == nil && count > 0
}

var ErrInvalidRole = errors.New("invalid assignable role")
var ErrLastAdmin = errors.New("cannot revoke the last admin")

func (r *Repository) Create(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *Repository) MarkEmailVerified(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("id = ?", id).
		Update("is_verified", true).Error
}

func (r *Repository) Update(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Where("id = ?", user.ID).Save(user).Error
}

// IsUniqueViolation reports whether err is a Postgres unique-constraint failure.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return false
}
