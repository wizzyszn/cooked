package recipe

import (
	"github.com/google/uuid"
	appdb "github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestRecipeVersionLifecycleAndAccess(t *testing.T) {
	database := recipeDB(t)
	author := uuid.New()
	database.Exec("INSERT INTO users(id,email,name,user_name,is_verified,hash_pass) VALUES (?,'author@recipe.test','Author','recipe_author',true,'hash')", author)
	category, region, dish := uuid.New(), uuid.New(), uuid.New()
	database.Exec("INSERT INTO categories(id,name,slug) VALUES (?,'Main','main')", category)
	database.Exec("INSERT INTO regions(id,name,slug) VALUES (?,'Africa','africa')", region)
	database.Exec("INSERT INTO delicacies(id,name,description,category_id,status,published_at) VALUES (?,'Test Dish','dish',?,'published',now())", dish, category)
	database.Exec("INSERT INTO delicacy_regions VALUES (?,?)", dish, region)
	svc := NewService(NewRepository(database))
	servings := 2
	q := 1.0
	iid := uuid.New()
	snap := Snapshot{Title: "First title", BaseServings: &servings, Ingredients: []IngredientInput{{ID: iid, Name: "Rice", Quantity: &q, Position: 0}}, Steps: []StepInput{{Title: "Cook", Instruction: "Boil rice", Action: "boil", Position: 0, IngredientEntryIDs: []uuid.UUID{iid}}}}
	r, e := svc.Create(t.Context(), author, CreateRequest{DelicacyID: dish, Visibility: domain.RecipeVisibilityPublic, Snapshot: snap})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Get(t.Context(), r.ID, nil, false, nil); e == nil {
		t.Fatal("unpublished recipe was public")
	}
	v, e := svc.Publish(t.Context(), r.ID, author, "publish:first")
	if e != nil {
		t.Fatal(e)
	}
	again, e := svc.Publish(t.Context(), r.ID, author, "publish:first")
	if e != nil || again.ID != v.ID {
		t.Fatalf("idempotent publish=%#v err=%v", again, e)
	}
	if e = database.Model(&domain.RecipeVersion{}).Where("id=?", v.ID).Update("title", "mutated").Error; e == nil {
		t.Fatal("published version was mutable")
	}
	draft, e := svc.Draft(t.Context(), r.ID, author, false)
	if e != nil || draft.VersionNumber != 2 {
		t.Fatalf("draft=%#v err=%v", draft, e)
	}
	snap.Title = "Second title"
	snap.Ingredients[0].ID = draft.Ingredients[0].ID
	snap.Steps[0].IngredientEntryIDs = []uuid.UUID{draft.Ingredients[0].ID}
	if _, e = svc.UpdateDraft(t.Context(), r.ID, author, snap, false); e != nil {
		t.Fatal(e)
	}
	v2, e := svc.Publish(t.Context(), r.ID, author, "publish:second")
	if e != nil || v2.VersionNumber != 2 {
		t.Fatalf("v2=%#v err=%v", v2, e)
	}
	old, e := svc.GetVersion(t.Context(), v.ID, nil, false, nil)
	if e != nil || old.Title != "First title" {
		t.Fatalf("old snapshot changed: %#v err=%v", old, e)
	}
	if e = svc.Visibility(t.Context(), r.ID, author, domain.RecipeVisibilityPrivate); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Get(t.Context(), r.ID, nil, false, nil); e == nil {
		t.Fatal("private recipe visible to guest")
	}
	if _, e = svc.Get(t.Context(), r.ID, &author, false, nil); e != nil {
		t.Fatal("private recipe hidden from author")
	}
	if e = svc.Visibility(t.Context(), r.ID, author, domain.RecipeVisibilityUnlisted); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Get(t.Context(), r.ID, nil, false, nil); e != nil {
		t.Fatal("unlisted link unavailable")
	}
	if e = svc.Delete(t.Context(), r.ID, author); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.GetVersion(t.Context(), v.ID, &author, false, nil); e == nil {
		t.Fatal("deleted history remained readable")
	}
}
func TestConcurrentPublishUsesOneVersion(t *testing.T) {
	database := recipeDB(t)
	author := uuid.New()
	database.Exec("INSERT INTO users(id,email,name,user_name,is_verified,hash_pass) VALUES (?,'race@recipe.test','Race','recipe_race',true,'hash')", author)
	dish := uuid.New()
	database.Exec("INSERT INTO delicacies(id,name,description,status,published_at) VALUES (?,'Race Dish','dish','published',now())", dish)
	svc := NewService(NewRepository(database))
	n := 1
	q := 1.0
	i := uuid.New()
	r, e := svc.Create(t.Context(), author, CreateRequest{DelicacyID: dish, Snapshot: Snapshot{Title: "Race", BaseServings: &n, Ingredients: []IngredientInput{{ID: i, Name: "Salt", Quantity: &q}}, Steps: []StepInput{{Instruction: "Mix", Action: "other"}}}})
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	ids := make(chan uuid.UUID, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := svc.Publish(t.Context(), r.ID, author, "same-command")
			if e != nil {
				errs <- e
				return
			}
			ids <- v.ID
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	close(ids)
	var first uuid.UUID
	for id := range ids {
		if first == uuid.Nil {
			first = id
		} else if first != id {
			t.Fatal("concurrent publish returned different versions")
		}
	}
	var count int64
	database.Raw("SELECT count(*) FROM recipe_versions WHERE recipe_id=? AND lifecycle='published'", r.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("published count=%d", count)
	}
}
func recipeDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("COOKED_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("COOKED_TEST_DATABASE_URL is not configured")
	}
	base, e := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "cooked_recipe_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if e = base.Exec("CREATE SCHEMA " + schema).Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { base.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE") })
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	d, e := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e = appdb.Migrate(d); e != nil {
		t.Fatal(e)
	}
	return d
}
