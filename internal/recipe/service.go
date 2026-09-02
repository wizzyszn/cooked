package recipe

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/platform"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"math"
	"net/http"
	"strings"
	"time"
)

type Service struct{ repo *Repository }

func NewService(r *Repository) *Service { return &Service{repo: r} }

var actions = map[string]bool{"sauté": true, "boil": true, "simmer": true, "fry": true, "bake": true, "grill": true, "fold": true, "whisk": true, "chop": true, "marinate": true, "rest": true, "other": true}

func validate(s Snapshot, publish bool) error {
	if strings.TrimSpace(s.Title) == "" {
		return apperrors.ErrValidation
	}
	if s.BaseServings != nil && *s.BaseServings <= 0 {
		return apperrors.ErrValidation
	}
	seenI := map[int]bool{}
	ids := map[uuid.UUID]bool{}
	for _, i := range s.Ingredients {
		if i.Position < 0 || seenI[i.Position] || strings.TrimSpace(i.Name) == "" || (i.Quantity == nil && strings.TrimSpace(i.DisplayAmount) == "") {
			return apperrors.ErrValidation
		}
		seenI[i.Position] = true
		if i.ID != uuid.Nil {
			ids[i.ID] = true
		}
	}
	seenS := map[int]bool{}
	for _, x := range s.Steps {
		if x.Position < 0 || seenS[x.Position] || !actions[x.Action] || strings.TrimSpace(x.Instruction) == "" || (x.DurationSeconds != nil && *x.DurationSeconds < 0) {
			return apperrors.ErrValidation
		}
		seenS[x.Position] = true
		for _, id := range x.IngredientEntryIDs {
			if !ids[id] {
				return apperrors.ErrValidation
			}
		}
	}
	if publish && (s.BaseServings == nil || len(s.Ingredients) == 0 || len(s.Steps) == 0) {
		return apperrors.New("RECIPE_INCOMPLETE", "a publishable recipe needs servings, ingredients, and steps", http.StatusBadRequest)
	}
	return nil
}
func (s *Service) Create(ctx context.Context, author uuid.UUID, req CreateRequest) (*domain.Recipe, error) {
	if req.Visibility == "" {
		req.Visibility = domain.RecipeVisibilityPrivate
	}
	if !validVisibility(req.Visibility) || validate(req.Snapshot, false) != nil {
		return nil, apperrors.ErrValidation
	}
	var out *domain.Recipe
	e := db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		var n int64
		if e := tx.Raw("SELECT count(*) FROM delicacies WHERE id=? AND status='published'", req.DelicacyID).Scan(&n).Error; e != nil || n != 1 {
			return apperrors.ErrValidation
		}
		r := &domain.Recipe{UserID: author, DelicacyID: req.DelicacyID, Title: req.Snapshot.Title, Algo: "", Visibility: req.Visibility, ModerationStatus: domain.RecipeVisible}
		if e := tx.Create(r).Error; e != nil {
			return e
		}
		v := &domain.RecipeVersion{RecipeID: r.ID, VersionNumber: 1, Lifecycle: domain.RecipeVersionDraft, LegacyImageURLs: datatypes.JSON([]byte("[]"))}
		if e := tx.Create(v).Error; e != nil {
			return e
		}
		if e := writeSnapshot(ctx, tx, v, req.Snapshot); e != nil {
			return e
		}
		out = r
		return nil
	})
	return out, e
}
func (s *Service) Get(ctx context.Context, id uuid.UUID, viewer *uuid.UUID, staff bool, servings *int) (*domain.RecipeVersion, error) {
	r, e := s.repo.Recipe(ctx, id, false)
	if e != nil || r == nil || r.DeletedAt.Valid || r.ModerationStatus != domain.RecipeVisible {
		return nil, apperrors.ErrNotFound
	}
	owner := viewer != nil && *viewer == r.UserID
	if r.Visibility == domain.RecipeVisibilityPrivate && !owner && !staff {
		return nil, apperrors.ErrNotFound
	}
	if r.CurrentPublishedVersionID == nil {
		return nil, apperrors.ErrNotFound
	}
	v, e := s.repo.Version(ctx, *r.CurrentPublishedVersionID)
	if e != nil {
		return nil, e
	}
	scale(v, servings)
	return v, nil
}
func (s *Service) GetVersion(ctx context.Context, id uuid.UUID, viewer *uuid.UUID, staff bool, servings *int) (*domain.RecipeVersion, error) {
	v, e := s.repo.Version(ctx, id)
	if e != nil || v == nil {
		return nil, apperrors.ErrNotFound
	}
	r, e := s.repo.Recipe(ctx, v.RecipeID, false)
	if e != nil || r == nil || r.DeletedAt.Valid || r.ModerationStatus != domain.RecipeVisible {
		return nil, apperrors.ErrNotFound
	}
	owner := viewer != nil && *viewer == r.UserID
	if v.Lifecycle == domain.RecipeVersionDraft && !owner && !staff {
		return nil, apperrors.ErrNotFound
	}
	if r.Visibility == domain.RecipeVisibilityPrivate && !owner && !staff {
		return nil, apperrors.ErrNotFound
	}
	v.Outdated = r.CurrentPublishedVersionID != nil && *r.CurrentPublishedVersionID != v.ID
	scale(v, servings)
	return v, nil
}
func (s *Service) Draft(ctx context.Context, id, actor uuid.UUID, staff bool) (*domain.RecipeVersion, error) {
	r, e := s.repo.Recipe(ctx, id, false)
	if e != nil || r == nil {
		return nil, apperrors.ErrNotFound
	}
	if r.UserID != actor && !staff {
		return nil, apperrors.ErrForbidden
	}
	if v, e := s.repo.Draft(ctx, id); e != nil || v != nil {
		return v, e
	}
	if r.CurrentPublishedVersionID == nil {
		return nil, apperrors.ErrNotFound
	}
	var out *domain.RecipeVersion
	e = db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		r, _ := rr(tx).Recipe(ctx, id, true)
		if v, _ := rr(tx).Draft(ctx, id); v != nil {
			out = v
			return nil
		}
		cur, e := rr(tx).Version(ctx, *r.CurrentPublishedVersionID)
		if e != nil {
			return e
		}
		v := &domain.RecipeVersion{RecipeID: id, VersionNumber: cur.VersionNumber + 1, Lifecycle: domain.RecipeVersionDraft, Title: cur.Title, Summary: cur.Summary, BaseServings: cur.BaseServings, PrepTimeSeconds: cur.PrepTimeSeconds, CookTimeSeconds: cur.CookTimeSeconds, Difficulty: cur.Difficulty, Notes: cur.Notes, LegacyImageURLs: datatypes.JSON([]byte("[]"))}
		if e = tx.Create(v).Error; e != nil {
			return e
		}
		if e = tx.Exec("INSERT INTO recipe_version_ingredients(id,recipe_version_id,ingredient_id,name,quantity,measurement_unit_id,display_amount,substitution_note,position) SELECT gen_random_uuid(),?,ingredient_id,name,quantity,measurement_unit_id,display_amount,substitution_note,position FROM recipe_version_ingredients WHERE recipe_version_id=?", v.ID, cur.ID).Error; e != nil {
			return e
		}
		if e = tx.Exec("INSERT INTO recipe_version_steps(id,recipe_version_id,position,title,instruction,action,duration_seconds,technique_tags) SELECT gen_random_uuid(),?,position,title,instruction,action,duration_seconds,technique_tags FROM recipe_version_steps WHERE recipe_version_id=?", v.ID, cur.ID).Error; e != nil {
			return e
		}
		if e = tx.Exec("INSERT INTO recipe_step_ingredients(step_id,ingredient_entry_id) SELECT ns.id,ni.id FROM recipe_step_ingredients x JOIN recipe_version_steps os ON os.id=x.step_id JOIN recipe_version_ingredients oi ON oi.id=x.ingredient_entry_id JOIN recipe_version_steps ns ON ns.recipe_version_id=? AND ns.position=os.position JOIN recipe_version_ingredients ni ON ni.recipe_version_id=? AND ni.position=oi.position", v.ID, v.ID).Error; e != nil {
			return e
		}
		if e = tx.Exec("INSERT INTO recipe_version_tags SELECT ?,tag_id FROM recipe_version_tags WHERE recipe_version_id=?", v.ID, cur.ID).Error; e != nil {
			return e
		}
		if e = tx.Exec("INSERT INTO recipe_version_media(recipe_version_id,media_asset_id,purpose,step_id,position) SELECT ?,m.media_asset_id,m.purpose,ns.id,m.position FROM recipe_version_media m LEFT JOIN recipe_version_steps os ON os.id=m.step_id LEFT JOIN recipe_version_steps ns ON ns.recipe_version_id=? AND ns.position=os.position WHERE m.recipe_version_id=?", v.ID, v.ID, cur.ID).Error; e != nil {
			return e
		}
		out = v
		return nil
	})
	if e != nil || out == nil {
		return out, e
	}
	return s.repo.Version(ctx, out.ID)
}
func (s *Service) UpdateDraft(ctx context.Context, id, actor uuid.UUID, snap Snapshot, staff bool) (*domain.RecipeVersion, error) {
	if e := validate(snap, false); e != nil {
		return nil, e
	}
	r, e := s.repo.Recipe(ctx, id, false)
	if e != nil || r == nil {
		return nil, apperrors.ErrNotFound
	}
	if r.UserID != actor && !staff {
		return nil, apperrors.ErrForbidden
	}
	v, e := s.repo.Draft(ctx, id)
	if e != nil || v == nil {
		return nil, apperrors.ErrNotFound
	}
	e = db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error { return writeSnapshot(ctx, tx, v, snap) })
	if e != nil {
		return nil, e
	}
	return s.repo.Version(ctx, v.ID)
}
func (s *Service) Publish(ctx context.Context, id, actor uuid.UUID, key string) (*domain.RecipeVersion, error) {
	if _, e := platform.ParseIdempotencyKey(key); e != nil {
		return nil, apperrors.ErrValidation
	}
	var out *domain.RecipeVersion
	e := db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		r, e := rr(tx).Recipe(ctx, id, true)
		if e != nil || r == nil {
			return apperrors.ErrNotFound
		}
		if r.UserID != actor {
			return apperrors.ErrForbidden
		}
		var existingRaw string
		if e = tx.Raw("SELECT published_version_id::text FROM recipe_publish_commands WHERE recipe_id=? AND idempotency_key=?", id, key).Scan(&existingRaw).Error; e == nil && existingRaw != "" {
			existing, _ := uuid.Parse(existingRaw)
			out, _ = rr(tx).Version(ctx, existing)
			return nil
		}
		v, e := rr(tx).Draft(ctx, id)
		if e != nil || v == nil {
			return apperrors.ErrConflict
		}
		snap := snapshot(v)
		if e = validate(snap, true); e != nil {
			return e
		}
		if e = validateMedia(tx, v.ID, actor); e != nil {
			return e
		}
		now := time.Now().UTC()
		if e = tx.Table("recipe_versions").Where("id=?", v.ID).Updates(map[string]any{"lifecycle": domain.RecipeVersionPublished, "published_at": now, "updated_at": now}).Error; e != nil {
			return e
		}
		if e = tx.Model(r).Updates(map[string]any{"current_published_version_id": v.ID, "title": v.Title}).Error; e != nil {
			return e
		}
		if e = tx.Exec("INSERT INTO recipe_publish_commands(recipe_id,idempotency_key,published_version_id) VALUES (?,?,?)", id, key, v.ID).Error; e != nil {
			return e
		}
		out = v
		out.Lifecycle = domain.RecipeVersionPublished
		out.PublishedAt = &now
		return nil
	})
	return out, e
}
func validVisibility(v domain.RecipeVisibility) bool {
	return v == domain.RecipeVisibilityPublic || v == domain.RecipeVisibilityPrivate || v == domain.RecipeVisibilityUnlisted
}
func (s *Service) Visibility(ctx context.Context, id, actor uuid.UUID, v domain.RecipeVisibility) error {
	if !validVisibility(v) {
		return apperrors.ErrValidation
	}
	return s.repo.DB().WithContext(ctx).Model(&domain.Recipe{}).Where("id=? AND user_id=?", id, actor).Update("visibility", v).Error
}
func (s *Service) Delete(ctx context.Context, id, actor uuid.UUID) error {
	return s.repo.DB().WithContext(ctx).Where("id=? AND user_id=?", id, actor).Delete(&domain.Recipe{}).Error
}
func scale(v *domain.RecipeVersion, requested *int) {
	for i := range v.Ingredients {
		v.Ingredients[i].Scalable = v.Ingredients[i].Quantity != nil
		if requested != nil && *requested > 0 && v.BaseServings != nil && *v.BaseServings > 0 && v.Ingredients[i].Quantity != nil {
			x := math.Round(*v.Ingredients[i].Quantity*float64(*requested)/float64(*v.BaseServings)*100) / 100
			v.Ingredients[i].ScaledQuantity = &x
		}
	}
}
func snapshot(v *domain.RecipeVersion) Snapshot {
	x := Snapshot{Title: v.Title, Summary: v.Summary, BaseServings: v.BaseServings, PrepTimeSeconds: v.PrepTimeSeconds, CookTimeSeconds: v.CookTimeSeconds, Difficulty: v.Difficulty, Notes: v.Notes}
	for _, i := range v.Ingredients {
		x.Ingredients = append(x.Ingredients, IngredientInput{ID: i.ID, IngredientID: i.IngredientID, Name: i.Name, Quantity: i.Quantity, MeasurementUnitID: i.MeasurementUnitID, DisplayAmount: i.DisplayAmount, SubstitutionNote: i.SubstitutionNote, Position: i.Position})
	}
	for _, s := range v.Steps {
		x.Steps = append(x.Steps, StepInput{ID: s.ID, Position: s.Position, Title: s.Title, Instruction: s.Instruction, Action: s.Action, DurationSeconds: s.DurationSeconds, TechniqueTags: s.TechniqueTags})
	}
	return x
}
func validateMedia(tx *gorm.DB, version, owner uuid.UUID) error {
	var bad int64
	tx.Raw("SELECT count(*) FROM recipe_version_media rvm JOIN media_assets m ON m.id=rvm.media_asset_id WHERE rvm.recipe_version_id=? AND (m.owner_id<>? OR m.processing_status<>'ready' OR m.moderation_status<>'approved' OR (rvm.purpose='cover' AND m.purpose<>'recipe_cover') OR (rvm.purpose='step' AND m.purpose<>'step_image'))", version, owner).Scan(&bad)
	if bad > 0 {
		return apperrors.New("MEDIA_NOT_READY", "all recipe media must be owned, processed, and approved", http.StatusConflict)
	}
	return nil
}
func writeSnapshot(ctx context.Context, tx *gorm.DB, v *domain.RecipeVersion, s Snapshot) error {
	if e := tx.WithContext(ctx).Model(v).Updates(map[string]any{"title": strings.TrimSpace(s.Title), "summary": s.Summary, "base_servings": s.BaseServings, "prep_time_seconds": s.PrepTimeSeconds, "cook_time_seconds": s.CookTimeSeconds, "difficulty": s.Difficulty, "notes": s.Notes}).Error; e != nil {
		return e
	}
	for _, table := range []string{"recipe_version_media", "recipe_version_tags", "recipe_step_ingredients", "recipe_version_steps", "recipe_version_ingredients"} {
		q := "DELETE FROM " + table
		if table == "recipe_step_ingredients" {
			q += " WHERE step_id IN (SELECT id FROM recipe_version_steps WHERE recipe_version_id=?)"
		} else if table == "recipe_version_media" || table == "recipe_version_tags" {
			q += " WHERE recipe_version_id=?"
		} else {
			q += " WHERE recipe_version_id=?"
		}
		if e := tx.Exec(q, v.ID).Error; e != nil {
			return e
		}
	}
	for _, i := range s.Ingredients {
		id := i.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		if e := tx.Create(&domain.RecipeVersionIngredient{ID: id, RecipeVersionID: v.ID, IngredientID: i.IngredientID, Name: strings.TrimSpace(i.Name), Quantity: i.Quantity, MeasurementUnitID: i.MeasurementUnitID, DisplayAmount: i.DisplayAmount, SubstitutionNote: i.SubstitutionNote, Position: i.Position}).Error; e != nil {
			return e
		}
	}
	for _, x := range s.Steps {
		id := x.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		techniques := pq.StringArray(x.TechniqueTags)
		if techniques == nil {
			techniques = pq.StringArray{}
		}
		if e := tx.Create(&domain.RecipeVersionStep{ID: id, RecipeVersionID: v.ID, Position: x.Position, Title: x.Title, Instruction: x.Instruction, Action: x.Action, DurationSeconds: x.DurationSeconds, TechniqueTags: techniques}).Error; e != nil {
			return e
		}
		for _, iid := range x.IngredientEntryIDs {
			if e := tx.Exec("INSERT INTO recipe_step_ingredients(step_id,ingredient_entry_id) VALUES (?,?)", id, iid).Error; e != nil {
				return e
			}
		}
		for p, mid := range x.MediaIDs {
			if e := tx.Exec("INSERT INTO recipe_version_media(recipe_version_id,media_asset_id,purpose,step_id,position) VALUES (?,?,'step',?,?)", v.ID, mid, id, p).Error; e != nil {
				return e
			}
		}
	}
	for _, tid := range s.TagIDs {
		if e := tx.Exec("INSERT INTO recipe_version_tags(recipe_version_id,tag_id) VALUES (?,?)", v.ID, tid).Error; e != nil {
			return e
		}
	}
	for p, mid := range s.CoverMediaIDs {
		if e := tx.Exec("INSERT INTO recipe_version_media(recipe_version_id,media_asset_id,purpose,position) VALUES (?,?,'cover',?)", v.ID, mid, p).Error; e != nil {
			return e
		}
	}
	return nil
}

var _ = errors.Is
