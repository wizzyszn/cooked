package review

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/notify"
	"github.com/wizzyszn/cooked/internal/platform"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

var reportReasons = map[string]bool{"spam": true, "harassment": true, "hate": true, "dangerous": true, "copyright": true, "misinformation": true, "other": true}

func validWrite(q WriteRequest) bool {
	return q.Taste >= 1 && q.Taste <= 5 && q.Clarity >= 1 && q.Clarity <= 5 && q.DifficultyAccuracy >= 1 && q.DifficultyAccuracy <= 5 && len(strings.TrimSpace(q.Comment)) <= 5000
}

func (s *Service) Create(ctx context.Context, actor, versionID uuid.UUID, key string, q WriteRequest) (*domain.Review, error) {
	if _, err := platform.ParseIdempotencyKey(key); err != nil || !validWrite(q) {
		return nil, apperrors.ErrValidation
	}
	var out domain.Review
	err := db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		var verified bool
		if err := tx.Raw("SELECT is_verified FROM users WHERE id=? AND deactivated_at IS NULL", actor).Scan(&verified).Error; err != nil {
			return err
		}
		if !verified {
			return apperrors.ErrEmailNotVerified
		}
		var priorRaw string
		if err := tx.Raw("SELECT review_id::text FROM review_create_commands WHERE user_id=? AND idempotency_key=?", actor, key).Scan(&priorRaw).Error; err != nil {
			return err
		}
		if priorRaw != "" {
			prior, err := uuid.Parse(priorRaw)
			if err != nil {
				return err
			}
			return tx.First(&out, "id=?", prior).Error
		}
		var row struct {
			RecipeID, AuthorID                uuid.UUID
			Lifecycle, Visibility, Moderation string
		}
		if err := tx.Raw(`SELECT v.recipe_id, r.user_id author_id, v.lifecycle, r.visibility, r.moderation_status moderation FROM recipe_versions v JOIN recipes r ON r.id=v.recipe_id WHERE v.id=? AND r.deleted_at IS NULL FOR UPDATE OF r`, versionID).Scan(&row).Error; err != nil {
			return err
		}
		if row.RecipeID == uuid.Nil || row.Lifecycle != "published" || row.Moderation != "visible" || row.Visibility == "private" {
			return apperrors.ErrNotFound
		}
		if row.AuthorID == actor {
			return apperrors.ErrForbidden
		}
		var completed int64
		if err := tx.Raw("SELECT count(*) FROM cook_sessions WHERE user_id=? AND recipe_version_id=? AND status='completed'", actor, versionID).Scan(&completed).Error; err != nil {
			return err
		}
		if completed == 0 {
			return apperrors.New("REVIEW_NOT_ELIGIBLE", "complete this recipe version before reviewing it", 403)
		}
		if q.PhotoMediaID != nil {
			var media int64
			if err := tx.Raw("SELECT count(*) FROM media_assets WHERE id=? AND owner_id=? AND purpose='review_photo' AND processing_status='ready' AND moderation_status='approved' AND deleted_at IS NULL", *q.PhotoMediaID, actor).Scan(&media).Error; err != nil {
				return err
			}
			if media != 1 {
				return apperrors.ErrValidation
			}
		}
		out = domain.Review{UserID: actor, RecipeID: row.RecipeID, RecipeVersionID: versionID, Taste: q.Taste, Clarity: q.Clarity, DifficultyAccuracy: q.DifficultyAccuracy, Comment: strings.TrimSpace(q.Comment), PhotoMediaID: q.PhotoMediaID, ModerationStatus: domain.ReviewVisible}
		if err := tx.Create(&out).Error; err != nil {
			return apperrors.ErrConflict
		}
		if err := tx.Exec("INSERT INTO review_create_commands(user_id,idempotency_key,review_id) VALUES (?,?,?)", actor, key, out.ID).Error; err != nil {
			return err
		}
		if err := recompute(tx, versionID); err != nil {
			return err
		}
		return notify.PersistOptional(ctx, tx, row.AuthorID, "activity", "new_review", "new-review:"+out.ID.String(), map[string]any{"review_id": out.ID, "recipe_id": row.RecipeID})
	})
	return &out, err
}

func (s *Service) Edit(ctx context.Context, actor, id uuid.UUID, q WriteRequest) (*domain.Review, error) {
	if !validWrite(q) {
		return nil, apperrors.ErrValidation
	}
	var out domain.Review
	err := db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&out, "id=?", id).Error; err != nil {
			return apperrors.ErrNotFound
		}
		if out.UserID != actor {
			return apperrors.ErrForbidden
		}
		if out.ModerationStatus == domain.ReviewRemoved {
			return apperrors.ErrNotFound
		}
		if q.PhotoMediaID != nil {
			var n int64
			tx.Raw("SELECT count(*) FROM media_assets WHERE id=? AND owner_id=? AND purpose='review_photo' AND processing_status='ready' AND moderation_status='approved' AND deleted_at IS NULL", *q.PhotoMediaID, actor).Scan(&n)
			if n != 1 {
				return apperrors.ErrValidation
			}
		}
		if err := tx.Model(&out).Updates(map[string]any{"taste": q.Taste, "clarity": q.Clarity, "difficulty_accuracy": q.DifficultyAccuracy, "comment": strings.TrimSpace(q.Comment), "photo_media_id": q.PhotoMediaID, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err := recompute(tx, out.RecipeVersionID); err != nil {
			return err
		}
		return tx.First(&out, "id=?", id).Error
	})
	return &out, err
}

func (s *Service) Get(ctx context.Context, id uuid.UUID, viewer *uuid.UUID, staff bool) (*domain.Review, error) {
	var out domain.Review
	if err := s.repo.DB().WithContext(ctx).First(&out, "id=?", id).Error; err != nil {
		return nil, apperrors.ErrNotFound
	}
	if out.ModerationStatus == domain.ReviewRemoved && !staff {
		return nil, apperrors.ErrNotFound
	}
	if out.ModerationStatus != domain.ReviewVisible && !staff && (viewer == nil || *viewer != out.UserID) {
		return nil, apperrors.ErrNotFound
	}
	if ok, _ := accessible(s.repo.DB().WithContext(ctx), out.RecipeID, viewer, staff); !ok {
		return nil, apperrors.ErrNotFound
	}
	return &out, nil
}

func (s *Service) List(ctx context.Context, versionID uuid.UUID, viewer *uuid.UUID, staff bool) (*ReviewList, error) {
	var recipeRaw string
	if err := s.repo.DB().WithContext(ctx).Raw("SELECT recipe_id::text FROM recipe_versions WHERE id=? AND lifecycle='published'", versionID).Scan(&recipeRaw).Error; err != nil || recipeRaw == "" {
		return nil, apperrors.ErrNotFound
	}
	recipeID, err := uuid.Parse(recipeRaw)
	if err != nil {
		return nil, err
	}
	if ok, _ := accessible(s.repo.DB().WithContext(ctx), recipeID, viewer, staff); !ok {
		return nil, apperrors.ErrNotFound
	}
	out := &ReviewList{Items: []domain.Review{}, Aggregate: domain.ReviewAggregate{RecipeVersionID: versionID}}
	if err := s.repo.DB().WithContext(ctx).Where("recipe_version_id=? AND moderation_status='visible'", versionID).Order("created_at DESC,id DESC").Find(&out.Items).Error; err != nil {
		return nil, err
	}
	_ = s.repo.DB().WithContext(ctx).First(&out.Aggregate, "recipe_version_id=?", versionID).Error
	return out, nil
}

func recompute(tx *gorm.DB, versionID uuid.UUID) error {
	return tx.Exec(`INSERT INTO recipe_version_review_aggregates(recipe_version_id,review_count,average_taste,average_clarity,average_difficulty_accuracy,updated_at)
	SELECT ?,count(*),COALESCE(avg(taste),0),COALESCE(avg(clarity),0),COALESCE(avg(difficulty_accuracy),0),now() FROM reviews WHERE recipe_version_id=? AND moderation_status='visible'
	ON CONFLICT(recipe_version_id) DO UPDATE SET review_count=excluded.review_count,average_taste=excluded.average_taste,average_clarity=excluded.average_clarity,average_difficulty_accuracy=excluded.average_difficulty_accuracy,updated_at=excluded.updated_at`, versionID, versionID).Error
}

func (s *Service) Reconcile(ctx context.Context, versionID uuid.UUID) error {
	return recompute(s.repo.DB().WithContext(ctx), versionID)
}

func accessible(tx *gorm.DB, recipeID uuid.UUID, viewer *uuid.UUID, staff bool) (bool, error) {
	var row struct {
		UserID                 uuid.UUID
		Visibility, Moderation string
		DeletedAt              *time.Time
	}
	if err := tx.Raw("SELECT user_id,visibility,moderation_status moderation,deleted_at FROM recipes WHERE id=?", recipeID).Scan(&row).Error; err != nil {
		return false, err
	}
	if row.UserID == uuid.Nil || row.DeletedAt != nil || row.Moderation != "visible" {
		return false, nil
	}
	return staff || (viewer != nil && *viewer == row.UserID) || row.Visibility == "public" || row.Visibility == "unlisted", nil
}
