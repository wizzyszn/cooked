package notify

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"gorm.io/gorm"
)

type Store interface {
	Create(ctx context.Context, n *domain.Notification) error
	MarkSent(ctx context.Context, id uuid.UUID, externalRef string) error
	MarkFailed(ctx context.Context, id uuid.UUID) error
	MarkSuppressed(ctx context.Context, id uuid.UUID) error
	ListPending(ctx context.Context, limit int) ([]domain.Notification, error)
}

type PostgresStore struct {
	db *gorm.DB
}

func NewStore(database *gorm.DB) *PostgresStore {
	return &PostgresStore{db: database}
}

func (s *PostgresStore) Create(ctx context.Context, n *domain.Notification) error {
	return s.db.WithContext(ctx).Create(n).Error
}

func (s *PostgresStore) MarkSent(ctx context.Context, id uuid.UUID, externalRef string) error {
	return s.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       domain.NotificationStatusSent,
			"external_ref": externalRef,
		}).Error
}

func (s *PostgresStore) MarkFailed(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("id = ?", id).
		Update("status", domain.NotificationStatusFailed).Error
}

func (s *PostgresStore) MarkSuppressed(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("id = ?", id).
		Update("status", domain.NotificationStatusSuppressed).Error
}

func (s *PostgresStore) ListPending(ctx context.Context, limit int) ([]domain.Notification, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []domain.Notification
	err := s.db.WithContext(ctx).
		Where("status = ?", domain.NotificationStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (s *PostgresStore) ClaimPending(ctx context.Context, worker string, limit int, now time.Time) ([]domain.Notification, error) {
	var rows []domain.Notification
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`SELECT * FROM notifications WHERE deleted_at IS NULL AND status IN ('pending','failed') AND next_attempt_at <= ? AND attempt_count < 5 AND (locked_at IS NULL OR locked_at < ?) ORDER BY next_attempt_at, created_at LIMIT ? FOR UPDATE SKIP LOCKED`, now, now.Add(-5*time.Minute), limit).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, len(rows))
		for i := range rows {
			ids[i] = rows[i].ID
		}
		return tx.Model(&domain.Notification{}).Where("id IN ?", ids).Updates(map[string]any{"locked_at": now, "locked_by": worker, "attempt_count": gorm.Expr("attempt_count + 1")}).Error
	})
	return rows, err
}
func (s *PostgresStore) StartAttempt(ctx context.Context, id uuid.UUID, number int, key string, now time.Time) error {
	return s.db.WithContext(ctx).Create(&domain.NotificationDeliveryAttempt{NotificationID: id, AttemptNumber: number, ProviderKey: key, Status: domain.NotificationStatusStarted, StartedAt: now}).Error
}
func (s *PostgresStore) FinishSent(ctx context.Context, id uuid.UUID, number int, key, ref string, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.NotificationDeliveryAttempt{}).Where("notification_id = ? AND attempt_number = ? AND provider_key = ?", id, number, key).Updates(map[string]any{"status": domain.NotificationStatusSent, "external_ref": ref, "completed_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.Notification{}).Where("id = ?", id).Updates(map[string]any{"status": domain.NotificationStatusSent, "external_ref": ref, "sent_at": now, "locked_at": nil, "locked_by": nil, "last_error": nil}).Error
	})
}
func (s *PostgresStore) FinishSuppressed(ctx context.Context, id uuid.UUID, number int, key string, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.NotificationDeliveryAttempt{}).Where("notification_id = ? AND attempt_number = ? AND provider_key = ?", id, number, key).Updates(map[string]any{"status": domain.NotificationStatusSuppressed, "completed_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.Notification{}).Where("id = ?", id).Updates(map[string]any{"status": domain.NotificationStatusSuppressed, "locked_at": nil, "locked_by": nil}).Error
	})
}
func (s *PostgresStore) FinishFailed(ctx context.Context, id uuid.UUID, number int, key, message string, retry bool, next, now time.Time) error {
	status := domain.NotificationStatusFailed
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.NotificationDeliveryAttempt{}).Where("notification_id = ? AND attempt_number = ? AND provider_key = ?", id, number, key).Updates(map[string]any{"status": domain.NotificationStatusFailed, "error": message, "completed_at": now}).Error; err != nil {
			return err
		}
		updates := map[string]any{"status": status, "last_error": message, "next_attempt_at": next, "locked_at": nil, "locked_by": nil}
		if !retry {
			updates["next_attempt_at"] = now.Add(100 * 365 * 24 * time.Hour)
		}
		return tx.Model(&domain.Notification{}).Where("id = ?", id).Updates(updates).Error
	})
}
