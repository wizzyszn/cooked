package discovery

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	appdb "github.com/wizzyszn/cooked/internal/db"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type fixture struct{ user, publicRecipe, privateRecipe, unlistedRecipe, hiddenRecipe uuid.UUID }

func TestDiscoveryVisibilityFiltersRecommendationsAndFavorites(t *testing.T) {
	database := discoveryDB(t)
	fx := seedDiscovery(t, database)
	svc := NewService(NewRepository(database))

	result, err := svc.Search(t.Context(), Filters{Query: "jollof", Difficulty: "easy", Category: "rice", Region: "west-africa", MaxSeconds: intPtr(3600), Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Dishes.Items) != 1 || len(result.Recipes.Items) != 1 || result.Recipes.Items[0].RecipeID != fx.publicRecipe {
		t.Fatalf("unexpected search: %#v", result)
	}

	recommended, err := svc.Recommendations(t.Context(), fx.user, Filters{Limit: 20})
	if err != nil || len(recommended.Items) != 1 || recommended.Items[0].RecipeID != fx.publicRecipe {
		t.Fatalf("recommendations=%#v err=%v", recommended, err)
	}

	for range 2 {
		if err = svc.Save(t.Context(), fx.user, fx.publicRecipe); err != nil {
			t.Fatal(err)
		}
	}
	for _, inaccessible := range []uuid.UUID{fx.privateRecipe, fx.unlistedRecipe, fx.hiddenRecipe} {
		if err = svc.Save(t.Context(), fx.user, inaccessible); err == nil {
			t.Fatalf("saved inaccessible recipe %s", inaccessible)
		}
	}
	saved, err := svc.Favorites(t.Context(), fx.user, Filters{Limit: 20})
	if err != nil || len(saved.Items) != 1 || saved.Items[0].RecipeID != fx.publicRecipe {
		t.Fatalf("favorites=%#v err=%v", saved, err)
	}
	if err = database.Exec("UPDATE recipes SET visibility='private' WHERE id=?", fx.publicRecipe).Error; err != nil {
		t.Fatal(err)
	}
	saved, err = svc.Favorites(t.Context(), fx.user, Filters{Limit: 20})
	if err != nil || len(saved.Items) != 0 {
		t.Fatalf("inaccessible favorite leaked: %#v err=%v", saved, err)
	}
	for range 2 {
		if err = svc.Unsave(t.Context(), fx.user, fx.publicRecipe); err != nil {
			t.Fatal(err)
		}
	}
}

func seedDiscovery(t *testing.T, db *gorm.DB) fixture {
	t.Helper()
	fx := fixture{user: uuid.New(), publicRecipe: uuid.New(), privateRecipe: uuid.New(), unlistedRecipe: uuid.New(), hiddenRecipe: uuid.New()}
	category, region, dish, generic := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	must(t, db, "INSERT INTO users(id,email,name,user_name,is_verified,hash_pass) VALUES (?,'discover@test.local','Cook','discover',true,'hash')", fx.user)
	must(t, db, "INSERT INTO categories(id,name,slug) VALUES (?,'Rice','rice')", category)
	must(t, db, "INSERT INTO regions(id,name,slug) VALUES (?,'West Africa','west-africa')", region)
	must(t, db, "INSERT INTO delicacies(id,name,description,category_id,status,published_at) VALUES (?,'Jollof Rice','dish',?,'published',now())", dish, category)
	must(t, db, "INSERT INTO delicacy_aliases(id,delicacy_id,name) VALUES (?,?,'Party Rice')", uuid.New(), dish)
	must(t, db, "INSERT INTO delicacy_regions(delicacy_id,region_id) VALUES (?,?)", dish, region)
	var dietaryRaw string
	if err := db.Raw("SELECT id::text FROM dietary_tags WHERE slug='vegetarian'").Scan(&dietaryRaw).Error; err != nil || dietaryRaw == "" {
		t.Fatalf("seeded dietary tag: id=%s err=%v", dietaryRaw, err)
	}
	dietary := uuid.MustParse(dietaryRaw)
	must(t, db, "INSERT INTO user_dietary_preferences(user_id,dietary_tag_id) VALUES (?,?)", fx.user, dietary)
	must(t, db, "INSERT INTO tags(id,name,slug,kind) VALUES (?,'Vegetarian','vegetarian','diet')", generic)
	for _, row := range []struct {
		id                     uuid.UUID
		visibility, moderation string
	}{{fx.publicRecipe, "public", "visible"}, {fx.privateRecipe, "private", "visible"}, {fx.unlistedRecipe, "unlisted", "visible"}, {fx.hiddenRecipe, "public", "hidden"}} {
		version := uuid.New()
		must(t, db, "INSERT INTO recipes(id,user_id,delicacy_id,title,algo,visibility,moderation_status) VALUES (?,?,?,'legacy','',?,?)", row.id, fx.user, dish, row.visibility, row.moderation)
		must(t, db, "INSERT INTO recipe_versions(id,recipe_id,version_number,lifecycle,title,summary,base_servings,prep_time_seconds,cook_time_seconds,difficulty) VALUES (?,?,1,'draft','Easy Jollof','simple',2,600,1200,'easy')", version, row.id)
		must(t, db, "INSERT INTO recipe_version_tags(recipe_version_id,tag_id) VALUES (?,?)", version, generic)
		must(t, db, "UPDATE recipe_versions SET lifecycle='published',published_at=now() WHERE id=?", version)
		must(t, db, "UPDATE recipes SET current_published_version_id=? WHERE id=?", version, row.id)
	}
	return fx
}
func must(t *testing.T, db *gorm.DB, q string, args ...any) {
	t.Helper()
	if err := db.Exec(q, args...).Error; err != nil {
		t.Fatal(err)
	}
}
func discoveryDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("COOKED_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("COOKED_TEST_DATABASE_URL is not configured")
	}
	base, e := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "cooked_discovery_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if e = base.Exec("CREATE SCHEMA " + schema).Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { base.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE") })
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	database, e := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e = appdb.Migrate(database); e != nil {
		t.Fatal(e)
	}
	return database
}

func TestCursorUsesStableTieBreaker(t *testing.T) {
	database := discoveryDB(t)
	fx := seedDiscovery(t, database)
	svc := NewService(NewRepository(database))
	second := uuid.New()
	version := uuid.New()
	var dishRaw string
	database.Raw("SELECT delicacy_id::text FROM recipes WHERE id=?", fx.publicRecipe).Scan(&dishRaw)
	dish := uuid.MustParse(dishRaw)
	must(t, database, "INSERT INTO recipes(id,user_id,delicacy_id,title,algo,visibility,moderation_status) VALUES (?,?,?,'second','','public','visible')", second, fx.user, dish)
	must(t, database, "INSERT INTO recipe_versions(id,recipe_id,version_number,lifecycle,title) VALUES (?,?,1,'draft','Second')", version, second)
	must(t, database, "UPDATE recipe_versions SET lifecycle='published',published_at=now() WHERE id=?", version)
	must(t, database, "UPDATE recipes SET current_published_version_id=? WHERE id=?", version, second)
	same := time.Now().UTC()
	must(t, database, "INSERT INTO favorites(user_id,recipe_id,created_at) VALUES (?,?,?),(?,?,?)", fx.user, fx.publicRecipe, same, fx.user, second, same)
	p, err := svc.Favorites(t.Context(), fx.user, Filters{Limit: 1})
	if err != nil || len(p.Items) != 1 || p.NextCursor == "" {
		t.Fatalf("first page=%#v err=%v", p, err)
	}
	p2, err := svc.Favorites(t.Context(), fx.user, Filters{Limit: 1, Cursor: p.NextCursor})
	if err != nil || len(p2.Items) != 1 || p.Items[0].RecipeID == p2.Items[0].RecipeID {
		t.Fatalf("second page=%#v err=%v", p2, err)
	}
}
