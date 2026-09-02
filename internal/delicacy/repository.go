package delicacy

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) DB() *gorm.DB          { return r.db }
func txRepo(db *gorm.DB) *Repository        { return &Repository{db: db} }
func (r *Repository) hydrate(q *gorm.DB) *gorm.DB {
	return q.Preload("Aliases").Preload("Regions").Preload("Category")
}
func (r *Repository) Public(ctx context.Context, id uuid.UUID) (*domain.Delicacy, error) {
	var d domain.Delicacy
	err := r.hydrate(r.db.WithContext(ctx)).Where("delicacies.id=? AND delicacies.status=?", id, domain.DelicacyPublished).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var x struct{ TargetID uuid.UUID }
		if e := r.db.WithContext(ctx).Table("delicacy_redirects").Where("source_id=?", id).Take(&x).Error; e == nil {
			return r.Public(ctx, x.TargetID)
		}
		return nil, nil
	}
	return &d, err
}
func (r *Repository) ByID(ctx context.Context, id uuid.UUID, lock bool) (*domain.Delicacy, error) {
	var d domain.Delicacy
	q := r.hydrate(r.db.WithContext(ctx))
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.First(&d, "delicacies.id=?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &d, err
}
func (r *Repository) List(ctx context.Context, category, region string, limit int) ([]domain.Delicacy, error) {
	var out []domain.Delicacy
	q := r.hydrate(r.db.WithContext(ctx)).Where("delicacies.status=?", domain.DelicacyPublished).Order("delicacies.published_at DESC, delicacies.id DESC").Limit(limit)
	if category != "" {
		q = q.Joins("JOIN categories c ON c.id=delicacies.category_id AND c.slug=?", category)
	}
	if region != "" {
		q = q.Joins("JOIN delicacy_regions dr ON dr.delicacy_id=delicacies.id JOIN regions rr ON rr.id=dr.region_id AND rr.slug=?", region)
	}
	return out, q.Find(&out).Error
}
func (r *Repository) Pending(ctx context.Context, limit int) ([]domain.Delicacy, error) {
	var out []domain.Delicacy
	return out, r.hydrate(r.db.WithContext(ctx)).Where("delicacies.status=?", domain.DelicacyPending).Order("delicacies.submitted_at, delicacies.id").Limit(limit).Find(&out).Error
}
func (r *Repository) Similar(ctx context.Context, name string, limit int) ([]domain.Delicacy, error) {
	var out []domain.Delicacy
	n := normalize(name)
	err := r.db.WithContext(ctx).Raw(`SELECT d.* FROM delicacies d LEFT JOIN delicacy_aliases a ON a.delicacy_id=d.id WHERE d.deleted_at IS NULL AND d.status IN ('pending','published') AND (lower(d.name)=? OR lower(a.name)=? OR similarity(lower(d.name),?)>=.35 OR similarity(lower(a.name),?)>=.35) GROUP BY d.id ORDER BY greatest(similarity(lower(d.name),?),coalesce(max(similarity(lower(a.name),?)),0)) DESC LIMIT ?`, n, n, n, n, n, n, limit).Scan(&out).Error
	return out, err
}
func (r *Repository) Save(ctx context.Context, d *domain.Delicacy, aliases []string, regions []uuid.UUID) error {
	if err := r.db.WithContext(ctx).Create(d).Error; err != nil {
		return err
	}
	return r.ReplaceRelations(ctx, d.ID, aliases, regions)
}
func (r *Repository) ReplaceRelations(ctx context.Context, id uuid.UUID, aliases []string, regions []uuid.UUID) error {
	if err := r.db.WithContext(ctx).Where("delicacy_id=?", id).Delete(&domain.DelicacyAlias{}).Error; err != nil {
		return err
	}
	for _, a := range aliases {
		if err := r.db.WithContext(ctx).Create(&domain.DelicacyAlias{ID: uuid.New(), DelicacyID: id, Name: a}).Error; err != nil {
			return err
		}
	}
	if err := r.db.WithContext(ctx).Exec("DELETE FROM delicacy_regions WHERE delicacy_id=?", id).Error; err != nil {
		return err
	}
	for _, rid := range regions {
		if err := r.db.WithContext(ctx).Exec("INSERT INTO delicacy_regions(delicacy_id,region_id) VALUES (?,?)", id, rid).Error; err != nil {
			return err
		}
	}
	return nil
}
func (r *Repository) Audit(ctx context.Context, a *domain.AuditLog) error {
	return r.db.WithContext(ctx).Create(a).Error
}
func (r *Repository) Taxonomies(ctx context.Context) (map[string]any, error) {
	var c []domain.Category
	var g []domain.Region
	var u []domain.MeasurementUnit
	if e := r.db.WithContext(ctx).Where("retired_at IS NULL").Order("name").Find(&c).Error; e != nil {
		return nil, e
	}
	if e := r.db.WithContext(ctx).Where("retired_at IS NULL").Order("name").Find(&g).Error; e != nil {
		return nil, e
	}
	if e := r.db.WithContext(ctx).Where("retired_at IS NULL").Order("name").Find(&u).Error; e != nil {
		return nil, e
	}
	var dietary []domain.DietaryTag
	if e := r.db.WithContext(ctx).Where("active=true").Order("name").Find(&dietary).Error; e != nil {
		return nil, e
	}
	return map[string]any{"categories": c, "regions": g, "measurement_units": u, "dietary_tags": dietary}, nil
}
func (r *Repository) UpsertTaxonomy(ctx context.Context, kind string, id *uuid.UUID, name, slug, symbol string) (any, error) {
	now := time.Now()
	switch kind {
	case "categories":
		x := domain.Category{Name: name, Slug: slug, UpdatedAt: now}
		if id != nil {
			x.ID = *id
			return x, r.db.WithContext(ctx).Save(&x).Error
		}
		x.ID = uuid.New()
		return x, r.db.WithContext(ctx).Create(&x).Error
	case "regions":
		x := domain.Region{Name: name, Slug: slug, UpdatedAt: now}
		if id != nil {
			x.ID = *id
			return x, r.db.WithContext(ctx).Save(&x).Error
		}
		x.ID = uuid.New()
		return x, r.db.WithContext(ctx).Create(&x).Error
	case "measurement-units":
		x := domain.MeasurementUnit{Name: name, Symbol: symbol, UpdatedAt: now}
		if id != nil {
			x.ID = *id
			return x, r.db.WithContext(ctx).Save(&x).Error
		}
		x.ID = uuid.New()
		return x, r.db.WithContext(ctx).Create(&x).Error
	case "dietary-tags":
		x := domain.DietaryTag{ID: uuid.New(), Name: name, Slug: slug, Active: true, CreatedAt: now}
		if id != nil {
			return nil, r.db.WithContext(ctx).Model(&domain.DietaryTag{}).Where("id=?", *id).Updates(map[string]any{"name": name, "slug": slug, "active": true}).Error
		}
		return x, r.db.WithContext(ctx).Create(&x).Error
	}
	return nil, errors.New("invalid taxonomy")
}
func (r *Repository) RetireTaxonomy(ctx context.Context, kind string, id uuid.UUID) error {
	table := map[string]string{"categories": "categories", "regions": "regions", "measurement-units": "measurement_units"}[kind]
	if kind == "dietary-tags" {
		return r.db.WithContext(ctx).Model(&domain.DietaryTag{}).Where("id=?", id).Update("active", false).Error
	}
	if table == "" {
		return errors.New("invalid taxonomy")
	}
	return r.db.WithContext(ctx).Table(table).Where("id=?", id).Update("retired_at", time.Now()).Error
}
