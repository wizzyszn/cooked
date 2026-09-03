package discovery

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/platform"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func normalizeFilters(f Filters) (Filters, error) {
	f.Query = strings.TrimSpace(f.Query)
	f.Dietary = strings.ToLower(strings.TrimSpace(f.Dietary))
	f.Difficulty = strings.ToLower(strings.TrimSpace(f.Difficulty))
	f.Category = strings.ToLower(strings.TrimSpace(f.Category))
	f.Region = strings.ToLower(strings.TrimSpace(f.Region))
	if f.Limit == 0 {
		f.Limit = 20
	}
	if f.Limit < 1 || f.Limit > 50 {
		return f, apperrors.ErrValidation
	}
	if f.Difficulty != "" && f.Difficulty != "easy" && f.Difficulty != "medium" && f.Difficulty != "hard" {
		return f, apperrors.ErrValidation
	}
	if f.MaxSeconds != nil && *f.MaxSeconds < 0 {
		return f, apperrors.ErrValidation
	}
	if f.Cursor != "" {
		if _, err := platform.DecodeCursor(f.Cursor); err != nil {
			return f, apperrors.ErrValidation
		}
	}
	for _, cursor := range []string{f.DishCursor, f.RecipeCursor} {
		if cursor != "" {
			if _, err := platform.DecodeCursor(cursor); err != nil {
				return f, apperrors.ErrValidation
			}
		}
	}
	return f, nil
}

func recipePage(items []RecipeCard, limit int) (RecipePage, error) {
	p := RecipePage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		p.Items = items[:limit]
		cursor, err := platform.EncodeCursor(platform.Cursor{Timestamp: last.CursorAt, ID: last.CursorID})
		if err != nil {
			return p, err
		}
		p.NextCursor = cursor
	}
	if p.Items == nil {
		p.Items = []RecipeCard{}
	}
	return p, nil
}
func dishPage(items []DishCard, limit int) (DishPage, error) {
	p := DishPage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		p.Items = items[:limit]
		cursor, err := platform.EncodeCursor(platform.Cursor{Timestamp: last.CursorAt, ID: last.CursorID})
		if err != nil {
			return p, err
		}
		p.NextCursor = cursor
	}
	if p.Items == nil {
		p.Items = []DishCard{}
	}
	return p, nil
}
func (s *Service) Search(ctx context.Context, f Filters) (SearchResult, error) {
	f, err := normalizeFilters(f)
	if err != nil {
		return SearchResult{}, err
	}
	dishFilters := f
	dishFilters.Cursor = f.DishCursor
	recipeFilters := f
	recipeFilters.Cursor = f.RecipeCursor
	dishes, err := s.repo.Dishes(ctx, dishFilters)
	if err != nil {
		return SearchResult{}, err
	}
	recipes, err := s.repo.Recipes(ctx, recipeFilters, nil)
	if err != nil {
		return SearchResult{}, err
	}
	dp, err := dishPage(dishes, f.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	rp, err := recipePage(recipes, f.Limit)
	return SearchResult{Dishes: dp, Recipes: rp}, err
}
func (s *Service) Browse(ctx context.Context, f Filters) (DishPage, error) {
	f.Query = ""
	f, err := normalizeFilters(f)
	if err != nil {
		return DishPage{}, err
	}
	items, err := s.repo.Dishes(ctx, f)
	if err != nil {
		return DishPage{}, err
	}
	return dishPage(items, f.Limit)
}
func (s *Service) Recent(ctx context.Context, f Filters) (DishPage, error) { return s.Browse(ctx, f) }
func (s *Service) Recommendations(ctx context.Context, userID uuid.UUID, f Filters) (RecipePage, error) {
	f.Query = ""
	f, err := normalizeFilters(f)
	if err != nil {
		return RecipePage{}, err
	}
	prefs, err := s.repo.Preferences(ctx, userID)
	if err != nil {
		return RecipePage{}, err
	}
	if len(prefs) == 0 {
		return RecipePage{Items: []RecipeCard{}}, nil
	}
	items, err := s.repo.Recipes(ctx, f, prefs)
	if err != nil {
		return RecipePage{}, err
	}
	return recipePage(items, f.Limit)
}
func (s *Service) Save(ctx context.Context, userID, recipeID uuid.UUID) error {
	ok, err := s.repo.Save(ctx, userID, recipeID)
	if err != nil {
		return err
	}
	if !ok {
		return apperrors.ErrNotFound
	}
	return nil
}
func (s *Service) Unsave(ctx context.Context, userID, recipeID uuid.UUID) error {
	return s.repo.Unsave(ctx, userID, recipeID)
}
func (s *Service) Favorites(ctx context.Context, userID uuid.UUID, f Filters) (RecipePage, error) {
	f, err := normalizeFilters(f)
	if err != nil {
		return RecipePage{}, err
	}
	items, err := s.repo.Favorites(ctx, userID, f)
	if err != nil {
		return RecipePage{}, err
	}
	return recipePage(items, f.Limit)
}

func (s *Service) Trending(ctx context.Context, f Filters) (RecipePage, error) {
	f.Query = ""
	f, err := normalizeFilters(f)
	if err != nil {
		return RecipePage{}, err
	}
	items, err := s.repo.Trending(ctx, f)
	if err != nil {
		return RecipePage{}, err
	}
	return recipePage(items, f.Limit)
}
