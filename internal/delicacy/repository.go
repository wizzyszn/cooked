package delicacy

import (
	"context"
	"errors"

	"github.com/wizzyszn/cooked/internal/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) GetDelicacyByName(ctx context.Context, name string) (*domain.Delicacy, error) {
	var delicacy domain.Delicacy

	err := r.db.WithContext(ctx).Where("lower(name) = ? AND deleted_at IS NULL", name).First(&delicacy).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &delicacy, err
}

func (r *Repository) CreateDelicacy(ctx context.Context, row *domain.Delicacy) error {
	return r.db.WithContext(ctx).Create(row).Error
}
