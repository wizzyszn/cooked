package media

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Create(ctx context.Context, asset *domain.MediaAsset) error {
	return r.db.WithContext(ctx).Create(asset).Error
}
func (r *Repository) Find(ctx context.Context, id uuid.UUID) (*domain.MediaAsset, error) {
	var asset domain.MediaAsset
	err := r.db.WithContext(ctx).Preload("Variants").First(&asset, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &asset, err
}
func (r *Repository) CanAccessRecipeMedia(ctx context.Context, mediaID, requester uuid.UUID) (bool, error) {
	var n int64
	e := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM recipe_version_media rvm JOIN recipe_versions v ON v.id=rvm.recipe_version_id JOIN recipes r ON r.id=v.recipe_id WHERE rvm.media_asset_id=? AND r.deleted_at IS NULL AND r.moderation_status='visible' AND (r.user_id=? OR r.visibility IN ('public','unlisted') OR EXISTS(SELECT 1 FROM user_roles ur WHERE ur.user_id=? AND ur.role IN ('moderator','admin'))) AND (v.lifecycle='published' OR r.user_id=? OR EXISTS(SELECT 1 FROM user_roles ur WHERE ur.user_id=? AND ur.role IN ('moderator','admin')))`, mediaID, requester, requester, requester, requester).Scan(&n).Error
	return n > 0, e
}
func (r *Repository) MarkUploaded(ctx context.Context, id, owner uuid.UUID, size int64, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&domain.MediaAsset{}).Where("id = ? AND owner_id = ? AND processing_status = ? AND expires_at > ?", id, owner, domain.MediaAwaitingUpload, now).Updates(map[string]any{"processing_status": domain.MediaUploaded, "byte_size": size, "uploaded_at": now, "next_attempt_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) Claim(ctx context.Context, worker string, limit int, now time.Time) ([]domain.MediaAsset, error) {
	var rows []domain.MediaAsset
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`SELECT * FROM media_assets WHERE deleted_at IS NULL AND processing_status IN ('uploaded','retry') AND next_attempt_at <= ? AND (locked_at IS NULL OR locked_at < ?) ORDER BY next_attempt_at, created_at LIMIT ? FOR UPDATE SKIP LOCKED`, now, now.Add(-5*time.Minute), limit).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, len(rows))
		for i := range rows {
			ids[i] = rows[i].ID
		}
		return tx.Model(&domain.MediaAsset{}).Where("id IN ?", ids).Updates(map[string]any{"processing_status": domain.MediaProcessing, "locked_at": now, "locked_by": worker, "attempt_count": gorm.Expr("attempt_count + 1")}).Error
	})
	return rows, err
}
func (r *Repository) Complete(ctx context.Context, id uuid.UUID, mime, checksum string, size int64, width, height int, variants []domain.MediaVariant, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("media_asset_id = ?", id).Delete(&domain.MediaVariant{}).Error; err != nil {
			return err
		}
		for i := range variants {
			variants[i].MediaAssetID = id
			if err := tx.Create(&variants[i]).Error; err != nil {
				return err
			}
		}
		return tx.Model(&domain.MediaAsset{}).Where("id = ?", id).Updates(map[string]any{"decoded_mime_type": mime, "checksum_sha256": checksum, "byte_size": size, "width": width, "height": height, "processing_status": domain.MediaReady, "moderation_status": domain.MediaModerationApproved, "processed_at": now, "locked_at": nil, "locked_by": nil, "last_error": nil}).Error
	})
}
func (r *Repository) Fail(ctx context.Context, id uuid.UUID, retry bool, message string, next time.Time) error {
	status := domain.MediaFailed
	moderation := domain.MediaModerationRejected
	if retry {
		status = domain.MediaRetry
		moderation = domain.MediaModerationPending
	}
	return r.db.WithContext(ctx).Model(&domain.MediaAsset{}).Where("id = ?", id).Updates(map[string]any{"processing_status": status, "moderation_status": moderation, "last_error": message, "next_attempt_at": next, "locked_at": nil, "locked_by": nil}).Error
}
func (r *Repository) ClaimOrphans(ctx context.Context, limit int, now time.Time) ([]domain.MediaAsset, error) {
	var rows []domain.MediaAsset
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("processing_status = ? AND expires_at <= ?", domain.MediaAwaitingUpload, now).Limit(limit).Find(&rows).Error
	return rows, err
}
func (r *Repository) MarkDeleted(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&domain.MediaAsset{}).Where("id = ?", id).Updates(map[string]any{"processing_status": domain.MediaDeleted, "deleted_at": time.Now().UTC()}).Error
}
