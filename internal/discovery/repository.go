package discovery

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/platform"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

const publicRecipeFrom = `
FROM recipes r
JOIN recipe_versions v ON v.id = r.current_published_version_id AND v.lifecycle = 'published'
JOIN delicacies d ON d.id = r.delicacy_id AND d.status = 'published' AND d.deleted_at IS NULL
LEFT JOIN categories c ON c.id = d.category_id
WHERE r.visibility = 'public' AND r.moderation_status = 'visible' AND r.deleted_at IS NULL`

func (r *Repository) Recipes(ctx context.Context, f Filters, preferences []string) ([]RecipeCard, error) {
	selectSQL := `SELECT r.id recipe_id, v.id version_id, v.title, v.summary, v.difficulty::text difficulty,
	 v.prep_time_seconds prep_seconds, v.cook_time_seconds cook_seconds, v.published_at,
	 d.id delicacy_id, d.name delicacy_name, c.slug category_slug, v.published_at cursor_at, v.id cursor_id,
	 COALESCE((SELECT array_agg(t.slug ORDER BY t.slug) FROM recipe_version_tags rvt JOIN tags t ON t.id=rvt.tag_id WHERE rvt.recipe_version_id=v.id AND t.kind='diet'),'{}') dietary_tags ` + publicRecipeFrom
	args := []any{}
	where := ""
	if f.Query != "" {
		where += " AND lower(v.title) LIKE ?"
		args = append(args, "%"+strings.ToLower(f.Query)+"%")
	}
	if f.Dietary != "" {
		where += " AND EXISTS (SELECT 1 FROM recipe_version_tags rvt JOIN tags t ON t.id=rvt.tag_id WHERE rvt.recipe_version_id=v.id AND t.kind='diet' AND t.slug=?)"
		args = append(args, f.Dietary)
	}
	if len(preferences) > 0 {
		where += " AND EXISTS (SELECT 1 FROM recipe_version_tags rvt JOIN tags t ON t.id=rvt.tag_id WHERE rvt.recipe_version_id=v.id AND t.kind='diet' AND t.slug IN ?)"
		args = append(args, preferences)
	}
	if f.Difficulty != "" {
		where += " AND v.difficulty=?"
		args = append(args, f.Difficulty)
	}
	if f.MaxSeconds != nil {
		where += " AND v.prep_time_seconds IS NOT NULL AND v.cook_time_seconds IS NOT NULL AND v.prep_time_seconds+v.cook_time_seconds<=?"
		args = append(args, *f.MaxSeconds)
	}
	if f.Category != "" {
		where += " AND c.slug=?"
		args = append(args, f.Category)
	}
	if f.Region != "" {
		where += " AND EXISTS (SELECT 1 FROM delicacy_regions dr JOIN regions rg ON rg.id=dr.region_id WHERE dr.delicacy_id=d.id AND rg.slug=?)"
		args = append(args, f.Region)
	}
	if f.Cursor != "" {
		cursor, err := platform.DecodeCursor(f.Cursor)
		if err != nil {
			return nil, err
		}
		where += " AND (v.published_at,v.id)<(?,?)"
		args = append(args, cursor.Timestamp, cursor.ID)
	}
	args = append(args, f.Limit+1)
	var out []RecipeCard
	err := r.db.WithContext(ctx).Raw(selectSQL+where+" ORDER BY v.published_at DESC,v.id DESC LIMIT ?", args...).Scan(&out).Error
	return out, err
}

func (r *Repository) Dishes(ctx context.Context, f Filters) ([]DishCard, error) {
	q := `SELECT d.id,d.name,d.description,c.slug category_slug,d.published_at,d.published_at cursor_at,d.id cursor_id,
	 COALESCE((SELECT array_agg(rg.slug ORDER BY rg.slug) FROM delicacy_regions dr JOIN regions rg ON rg.id=dr.region_id WHERE dr.delicacy_id=d.id),'{}') regions
	 FROM delicacies d LEFT JOIN categories c ON c.id=d.category_id WHERE d.status='published' AND d.deleted_at IS NULL`
	args := []any{}
	if f.Query != "" {
		q += " AND (lower(d.name) % ? OR lower(d.name) LIKE ? OR EXISTS (SELECT 1 FROM delicacy_aliases a WHERE a.delicacy_id=d.id AND (lower(a.name) % ? OR lower(a.name) LIKE ?)))"
		n := strings.ToLower(f.Query)
		args = append(args, n, "%"+n+"%", n, "%"+n+"%")
	}
	if f.Category != "" {
		q += " AND c.slug=?"
		args = append(args, f.Category)
	}
	if f.Region != "" {
		q += " AND EXISTS (SELECT 1 FROM delicacy_regions dr JOIN regions rg ON rg.id=dr.region_id WHERE dr.delicacy_id=d.id AND rg.slug=?)"
		args = append(args, f.Region)
	}
	if f.Cursor != "" {
		cursor, err := platform.DecodeCursor(f.Cursor)
		if err != nil {
			return nil, err
		}
		q += " AND (d.published_at,d.id)<(?,?)"
		args = append(args, cursor.Timestamp, cursor.ID)
	}
	args = append(args, f.Limit+1)
	var out []DishCard
	err := r.db.WithContext(ctx).Raw(q+" ORDER BY d.published_at DESC,d.id DESC LIMIT ?", args...).Scan(&out).Error
	return out, err
}

func (r *Repository) Save(ctx context.Context, userID, recipeID uuid.UUID) (bool, error) {
	var accessible int64
	if err := r.db.WithContext(ctx).Raw("SELECT count(*) "+publicRecipeFrom+" AND r.id=?", recipeID).Scan(&accessible).Error; err != nil {
		return false, err
	}
	if accessible == 0 {
		return false, nil
	}
	result := r.db.WithContext(ctx).Exec("INSERT INTO favorites(user_id,recipe_id) VALUES (?,?) ON CONFLICT (user_id,recipe_id) DO NOTHING", userID, recipeID)
	return true, result.Error
}

func (r *Repository) Unsave(ctx context.Context, userID, recipeID uuid.UUID) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM favorites WHERE user_id=? AND recipe_id=?", userID, recipeID).Error
}

func (r *Repository) Favorites(ctx context.Context, userID uuid.UUID, f Filters) ([]RecipeCard, error) {
	q := `SELECT r.id recipe_id,v.id version_id,v.title,v.summary,v.difficulty::text difficulty,v.prep_time_seconds prep_seconds,v.cook_time_seconds cook_seconds,v.published_at,d.id delicacy_id,d.name delicacy_name,c.slug category_slug,fav.created_at cursor_at,fav.recipe_id cursor_id,
	COALESCE((SELECT array_agg(t.slug ORDER BY t.slug) FROM recipe_version_tags rvt JOIN tags t ON t.id=rvt.tag_id WHERE rvt.recipe_version_id=v.id AND t.kind='diet'),'{}') dietary_tags
	FROM favorites fav JOIN recipes r ON r.id=fav.recipe_id JOIN recipe_versions v ON v.id=r.current_published_version_id AND v.lifecycle='published' JOIN delicacies d ON d.id=r.delicacy_id AND d.status='published' AND d.deleted_at IS NULL LEFT JOIN categories c ON c.id=d.category_id
	WHERE fav.user_id=? AND r.visibility='public' AND r.moderation_status='visible' AND r.deleted_at IS NULL`
	args := []any{userID}
	if f.Cursor != "" {
		cursor, err := platform.DecodeCursor(f.Cursor)
		if err != nil {
			return nil, err
		}
		q += " AND (fav.created_at,fav.recipe_id)<(?,?)"
		args = append(args, cursor.Timestamp, cursor.ID)
	}
	args = append(args, f.Limit+1)
	var out []RecipeCard
	err := r.db.WithContext(ctx).Raw(q+" ORDER BY fav.created_at DESC,fav.recipe_id DESC LIMIT ?", args...).Scan(&out).Error
	return out, err
}

func (r *Repository) Preferences(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var out []string
	err := r.db.WithContext(ctx).Raw("SELECT dt.slug FROM user_dietary_preferences udp JOIN dietary_tags dt ON dt.id=udp.dietary_tag_id AND dt.active=true WHERE udp.user_id=?", userID).Scan(&out).Error
	return out, err
}

func (r *Repository) Trending(ctx context.Context, f Filters) ([]RecipeCard, error) {
	q := `SELECT r.id recipe_id,v.id version_id,v.title,v.summary,v.difficulty::text difficulty,v.prep_time_seconds prep_seconds,v.cook_time_seconds cook_seconds,v.published_at,d.id delicacy_id,d.name delicacy_name,c.slug category_slug,to_timestamp(t.score) cursor_at,r.id cursor_id,t.score trend_score,COALESCE((SELECT array_agg(tag.slug ORDER BY tag.slug) FROM recipe_version_tags rvt JOIN tags tag ON tag.id=rvt.tag_id WHERE rvt.recipe_version_id=v.id AND tag.kind='diet'),'{}') dietary_tags FROM recipe_trend_scores t JOIN recipes r ON r.id=t.recipe_id JOIN recipe_versions v ON v.id=r.current_published_version_id AND v.lifecycle='published' JOIN delicacies d ON d.id=r.delicacy_id AND d.status='published' AND d.deleted_at IS NULL LEFT JOIN categories c ON c.id=d.category_id WHERE t.score>0 AND r.visibility='public' AND r.moderation_status='visible' AND r.deleted_at IS NULL`
	args := []any{}
	if f.Cursor != "" {
		cursor, err := platform.DecodeCursor(f.Cursor)
		if err != nil {
			return nil, err
		}
		q += " AND (t.score,r.id)<(?,?)"
		args = append(args, cursor.Timestamp.Unix(), cursor.ID)
	}
	args = append(args, f.Limit+1)
	var out []RecipeCard
	err := r.db.WithContext(ctx).Raw(q+" ORDER BY t.score DESC,r.id DESC LIMIT ?", args...).Scan(&out).Error
	return out, err
}
