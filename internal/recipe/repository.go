package recipe

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) DB() *gorm.DB          { return r.db }
func rr(db *gorm.DB) *Repository            { return &Repository{db: db} }
func (r *Repository) Recipe(ctx context.Context, id uuid.UUID, lock bool) (*domain.Recipe, error) {
	var x domain.Recipe
	q := r.db.WithContext(ctx)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	e := q.First(&x, "id=?", id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &x, e
}
func (r *Repository) Version(ctx context.Context, id uuid.UUID) (*domain.RecipeVersion, error) {
	var v domain.RecipeVersion
	e := r.db.WithContext(ctx).Preload("Ingredients", func(q *gorm.DB) *gorm.DB { return q.Order("position") }).Preload("Steps", func(q *gorm.DB) *gorm.DB { return q.Order("position") }).Preload("Tags").Preload("Media").First(&v, "id=?", id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if e == nil {
		var aggregate domain.ReviewAggregate
		if ae := r.db.WithContext(ctx).First(&aggregate, "recipe_version_id=?", v.ID).Error; ae == nil {
			v.ReviewAggregate = &aggregate
		} else if !errors.Is(ae, gorm.ErrRecordNotFound) {
			return nil, ae
		}
	}
	return &v, e
}
func (r *Repository) Draft(ctx context.Context, id uuid.UUID) (*domain.RecipeVersion, error) {
	var v domain.RecipeVersion
	e := r.db.WithContext(ctx).Where("recipe_id=? AND lifecycle='draft'", id).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return r.Version(ctx, v.ID)
}
