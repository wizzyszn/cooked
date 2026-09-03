package delicacy

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/notify"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var spaces = regexp.MustCompile(`\s+`)

func normalize(v string) string {
	return strings.ToLower(spaces.ReplaceAllString(strings.TrimSpace(v), " "))
}
func clean(req WriteRequest) (WriteRequest, error) {
	req.Name = spaces.ReplaceAllString(strings.TrimSpace(req.Name), " ")
	req.Description = strings.TrimSpace(req.Description)
	req.OriginNotes = strings.TrimSpace(req.OriginNotes)
	seen := map[string]bool{}
	aliases := req.Aliases[:0]
	for _, a := range req.Aliases {
		a = spaces.ReplaceAllString(strings.TrimSpace(a), " ")
		n := normalize(a)
		if n != "" && n != normalize(req.Name) && !seen[n] {
			seen[n] = true
			aliases = append(aliases, a)
		}
	}
	req.Aliases = aliases
	for i, c := range req.CountryCodes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if len(c) != 2 {
			return req, apperrors.New("INVALID_COUNTRY_CODE", "country codes must be ISO 3166-1 alpha-2", http.StatusBadRequest)
		}
		req.CountryCodes[i] = c
	}
	if req.CategoryID == nil || len(req.RegionIDs) == 0 {
		return req, apperrors.New("DISH_TAXONOMY_REQUIRED", "category_id and at least one region_id are required", http.StatusBadRequest)
	}
	return req, nil
}

type Service struct {
	log  *zap.SugaredLogger
	repo *Repository
}

func NewDelicacyService(log *zap.SugaredLogger, r *Repository) *Service {
	return &Service{log: log, repo: r}
}
func (s *Service) List(ctx context.Context, category, region string) ([]domain.Delicacy, error) {
	return s.repo.List(ctx, category, region, 50)
}
func (s *Service) Pending(ctx context.Context) ([]domain.Delicacy, error) {
	return s.repo.Pending(ctx, 50)
}
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*domain.Delicacy, error) {
	d, e := s.repo.Public(ctx, id)
	if e != nil {
		return nil, apperrors.Internal(s.log, "get dish", e)
	}
	if d == nil {
		return nil, apperrors.ErrNotFound
	}
	return d, nil
}
func (s *Service) Similar(ctx context.Context, name string) ([]domain.Delicacy, error) {
	if len(normalize(name)) < 3 {
		return nil, apperrors.ErrValidation
	}
	return s.repo.Similar(ctx, name, 8)
}
func (s *Service) Create(ctx context.Context, req WriteRequest, actor uuid.UUID, publish bool) (*domain.Delicacy, error) {
	var err error
	if req, err = clean(req); err != nil {
		return nil, err
	}
	similar, err := s.repo.Similar(ctx, req.Name, 8)
	if err != nil {
		return nil, apperrors.Internal(s.log, "find similar dishes", err)
	}
	if len(similar) > 0 && !req.ConfirmSimilar {
		return nil, apperrors.WithDetails(apperrors.New("SIMILAR_DISHES_FOUND", "similar dishes found; review and resubmit with confirm_similar", http.StatusConflict), similar)
	}
	now := time.Now().UTC()
	status := domain.DelicacyPending
	var publishedAt *time.Time
	if publish {
		status = domain.DelicacyPublished
		publishedAt = &now
	}
	d := &domain.Delicacy{Name: req.Name, Description: req.Description, CreatedBy: &actor, CategoryID: req.CategoryID, CoverMediaID: req.CoverMediaID, Status: status, CountryCodes: req.CountryCodes, OriginNotes: req.OriginNotes, SubmittedAt: &now, PublishedAt: publishedAt}
	err = db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		r := txRepo(tx)
		if e := r.Save(ctx, d, req.Aliases, req.RegionIDs); e != nil {
			return e
		}
		if publish {
			return r.Audit(ctx, audit(actor, "dish.created", d.ID, "", nil, d))
		}
		return nil
	})
	if err != nil {
		return nil, apperrors.Internal(s.log, "create dish", err)
	}
	return d, nil
}
func (s *Service) EditPending(ctx context.Context, id, actor uuid.UUID, req WriteRequest) (*domain.Delicacy, error) {
	var err error
	if req, err = clean(req); err != nil {
		return nil, err
	}
	d, err := s.repo.ByID(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, apperrors.ErrNotFound
	}
	if d.Status != domain.DelicacyPending || d.CreatedBy == nil || *d.CreatedBy != actor {
		return nil, apperrors.ErrForbidden
	}
	d.Name = req.Name
	d.Description = req.Description
	d.CategoryID = req.CategoryID
	d.CoverMediaID = req.CoverMediaID
	d.CountryCodes = req.CountryCodes
	d.OriginNotes = req.OriginNotes
	err = db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		if e := tx.WithContext(ctx).Save(d).Error; e != nil {
			return e
		}
		return txRepo(tx).ReplaceRelations(ctx, id, req.Aliases, req.RegionIDs)
	})
	return d, err
}
func (s *Service) Withdraw(ctx context.Context, id, actor uuid.UUID) error {
	return s.transition(ctx, id, actor, domain.DelicacyWithdrawn, "withdrawn by submitter", false)
}
func (s *Service) Moderate(ctx context.Context, id, actor uuid.UUID, status domain.DelicacyStatus, reason string) error {
	if status != domain.DelicacyPublished && status != domain.DelicacyRejected {
		return apperrors.ErrValidation
	}
	return s.transition(ctx, id, actor, status, strings.TrimSpace(reason), true)
}
func (s *Service) transition(ctx context.Context, id, actor uuid.UUID, status domain.DelicacyStatus, reason string, staff bool) error {
	if reason == "" {
		return apperrors.ErrValidation
	}
	return db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		r := txRepo(tx)
		d, e := r.ByID(ctx, id, true)
		if e != nil {
			return e
		}
		if d == nil {
			return apperrors.ErrNotFound
		}
		if d.Status != domain.DelicacyPending {
			return apperrors.ErrConflict
		}
		if !staff && (d.CreatedBy == nil || *d.CreatedBy != actor) {
			return apperrors.ErrForbidden
		}
		before := *d
		now := time.Now().UTC()
		d.Status = status
		d.ModerationReason = reason
		d.ModeratedAt = &now
		d.ModeratedBy = &actor
		if status == domain.DelicacyPublished {
			d.PublishedAt = &now
		}
		if e = tx.WithContext(ctx).Save(d).Error; e != nil {
			return e
		}
		if staff {
			if e = r.Audit(ctx, audit(actor, "dish."+string(status), id, reason, before, d)); e != nil {
				return e
			}
			if d.CreatedBy != nil {
				return notify.PersistOptional(ctx, tx, *d.CreatedBy, "activity", "dish_moderation_outcome", "dish-moderation:"+id.String()+":"+string(status), map[string]any{"dish_id": id, "dish_name": d.Name, "outcome": status, "reason": reason})
			}
		}
		return nil
	})
}
func (s *Service) Merge(ctx context.Context, sourceID, targetID, actor uuid.UUID, reason string) error {
	if sourceID == targetID || strings.TrimSpace(reason) == "" {
		return apperrors.ErrValidation
	}
	return db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		r := txRepo(tx)
		ids := []uuid.UUID{sourceID, targetID}
		if ids[1].String() < ids[0].String() {
			ids[0], ids[1] = ids[1], ids[0]
		}
		for _, id := range ids {
			if d, e := r.ByID(ctx, id, true); e != nil || d == nil {
				if e != nil {
					return e
				}
				return apperrors.ErrNotFound
			}
		}
		source, _ := r.ByID(ctx, sourceID, false)
		target, _ := r.ByID(ctx, targetID, false)
		if source.Status == domain.DelicacyRetired || target.Status != domain.DelicacyPublished {
			return apperrors.ErrConflict
		}
		before := map[string]any{"source": source, "target": target}
		if e := tx.WithContext(ctx).Exec("UPDATE recipes SET delicacy_id=? WHERE delicacy_id=?", targetID, sourceID).Error; e != nil {
			return e
		}
		if e := tx.WithContext(ctx).Exec("INSERT INTO delicacy_aliases(id,delicacy_id,name) SELECT gen_random_uuid(),?,name FROM delicacy_aliases WHERE delicacy_id=? ON CONFLICT DO NOTHING", targetID, sourceID).Error; e != nil {
			return e
		}
		if e := tx.WithContext(ctx).Exec("INSERT INTO delicacy_regions(delicacy_id,region_id) SELECT ?,region_id FROM delicacy_regions WHERE delicacy_id=? ON CONFLICT DO NOTHING", targetID, sourceID).Error; e != nil {
			return e
		}
		if e := tx.WithContext(ctx).Exec("UPDATE delicacies SET status='retired',moderated_by=?,moderated_at=now(),moderation_reason=? WHERE id=?", actor, reason, sourceID).Error; e != nil {
			return e
		}
		if e := tx.WithContext(ctx).Exec("INSERT INTO delicacy_aliases(id,delicacy_id,name) VALUES (gen_random_uuid(),?,?) ON CONFLICT DO NOTHING", targetID, source.Name).Error; e != nil {
			return e
		}
		if e := tx.WithContext(ctx).Exec("INSERT INTO delicacy_redirects(source_id,target_id,created_by) VALUES (?,?,?)", sourceID, targetID, actor).Error; e != nil {
			return e
		}
		return r.Audit(ctx, audit(actor, "dish.merged", sourceID, reason, before, map[string]any{"redirect_to": targetID}))
	})
}
func audit(actor uuid.UUID, action string, target uuid.UUID, reason string, before, after any) *domain.AuditLog {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	return &domain.AuditLog{ID: uuid.New(), ActorID: &actor, Action: action, TargetType: "delicacy", TargetID: &target, Reason: reason, BeforeJSON: b, AfterJSON: a}
}
func slug(v string) string { return strings.Trim(strings.ReplaceAll(normalize(v), " ", "-"), "-") }
func (s *Service) Taxonomies(ctx context.Context) (map[string]any, error) {
	return s.repo.Taxonomies(ctx)
}
func (s *Service) WriteTaxonomy(ctx context.Context, kind string, id *uuid.UUID, req TaxonomyRequest, actor uuid.UUID) (any, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Slug == "" {
		req.Slug = slug(req.Name)
	}
	var out interface{}
	err := db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		var e error
		out, e = txRepo(tx).UpsertTaxonomy(ctx, kind, id, req.Name, slug(req.Slug), strings.TrimSpace(req.Symbol))
		if e != nil {
			return e
		}
		target := actor
		if id != nil {
			target = *id
		}
		return txRepo(tx).Audit(ctx, audit(actor, "taxonomy."+kind+".upsert", target, "", nil, out))
	})
	return out, err
}
func (s *Service) RetireTaxonomy(ctx context.Context, kind string, id, actor uuid.UUID, reason string) error {
	return db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		r := txRepo(tx)
		if e := r.RetireTaxonomy(ctx, kind, id); e != nil {
			return e
		}
		return r.Audit(ctx, audit(actor, "taxonomy."+kind+".retired", id, reason, nil, map[string]any{"retired": true}))
	})
}
