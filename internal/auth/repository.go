package auth

import (
	"context"

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

func (r *Repository) CreateRefreshToken(ctx context.Context, refresh_token *domain.RefreshToken) error {
	if err := r.db.WithContext(ctx).Create(refresh_token).Error; err != nil {
		return err
	}
	return nil
}
