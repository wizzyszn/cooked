package discovery

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Filters struct {
	Query        string
	Dietary      string
	Difficulty   string
	Category     string
	Region       string
	MaxSeconds   *int
	Cursor       string
	DishCursor   string
	RecipeCursor string
	Limit        int
}

type RecipeCard struct {
	RecipeID     uuid.UUID      `json:"recipe_id"`
	VersionID    uuid.UUID      `json:"version_id"`
	Title        string         `json:"title"`
	Summary      string         `json:"summary"`
	Difficulty   *string        `json:"difficulty,omitempty"`
	PrepSeconds  *int           `json:"prep_time_seconds,omitempty"`
	CookSeconds  *int           `json:"cook_time_seconds,omitempty"`
	PublishedAt  time.Time      `json:"published_at"`
	DelicacyID   uuid.UUID      `json:"delicacy_id"`
	DelicacyName string         `json:"delicacy_name"`
	CategorySlug *string        `json:"category_slug,omitempty"`
	DietaryTags  pq.StringArray `gorm:"type:text[]" json:"dietary_tags,omitempty"`
	CursorAt     time.Time      `json:"-"`
	CursorID     uuid.UUID      `json:"-"`
}

type DishCard struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	CategorySlug *string        `json:"category_slug,omitempty"`
	Regions      pq.StringArray `gorm:"type:text[]" json:"regions,omitempty"`
	PublishedAt  time.Time      `json:"published_at"`
	CursorAt     time.Time      `json:"-"`
	CursorID     uuid.UUID      `json:"-"`
}

type RecipePage struct {
	Items      []RecipeCard `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

type DishPage struct {
	Items      []DishCard `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

type SearchResult struct {
	Dishes  DishPage   `json:"dishes"`
	Recipes RecipePage `json:"recipes"`
}
