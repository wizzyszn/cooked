package notify

import (
	"context"

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
