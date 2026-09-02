package delicacy

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	appdb "github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDishModerationLifecycle(t *testing.T) {
	database := dishTestDB(t)
	owner, staff := uuid.New(), uuid.New()
	for id, row := range map[uuid.UUID][]string{owner: {"owner@dish.test", "Owner", "dish_owner"}, staff: {"staff@dish.test", "Staff", "dish_staff"}} {
		if e := database.Exec("INSERT INTO users(id,email,name,user_name,is_verified,hash_pass) VALUES (?,?,?,?,true,'hash')", id, row[0], row[1], row[2]).Error; e != nil {
			t.Fatal(e)
		}
	}
	category, region := uuid.New(), uuid.New()
	if e := database.Exec("INSERT INTO categories(id,name,slug) VALUES (?,'Mains','mains')", category).Error; e != nil {
		t.Fatal(e)
	}
	if e := database.Exec("INSERT INTO regions(id,name,slug) VALUES (?,'West Africa','west-africa')", region).Error; e != nil {
		t.Fatal(e)
	}
	s := NewDelicacyService(nil, NewRepository(database))
	request := WriteRequest{Name: "Jollof Rice", Description: "Rice cooked in tomato sauce", CategoryID: &category, RegionIDs: []uuid.UUID{region}, Aliases: []string{"Party Jollof"}, CountryCodes: []string{"NG"}}
	pending, e := s.Create(t.Context(), request, owner, false)
	if e != nil || pending.Status != domain.DelicacyPending {
		t.Fatalf("submit=%#v error=%v", pending, e)
	}
	if rows, _ := s.List(t.Context(), "mains", "west-africa"); len(rows) != 0 {
		t.Fatal("pending dish was public")
	}
	_, e = s.Create(t.Context(), request, owner, false)
	var conflict *apperrors.AppError
	if !errors.As(e, &conflict) || conflict.Code != "SIMILAR_DISHES_FOUND" {
		t.Fatalf("duplicate error=%v", e)
	}
	if e = s.Moderate(t.Context(), pending.ID, staff, domain.DelicacyPublished, "verified canonical entry"); e != nil {
		t.Fatal(e)
	}
	if rows, _ := s.List(t.Context(), "mains", "west-africa"); len(rows) != 1 {
		t.Fatalf("published browse count=%d", len(rows))
	}
	assertAuditCount(t, database, pending.ID, "dish.published", 1)

	request.Name = "Benachin"
	request.Aliases = nil
	request.ConfirmSimilar = false
	source, e := s.Create(t.Context(), request, owner, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Moderate(t.Context(), source.ID, staff, domain.DelicacyPublished, "valid alternate name"); e != nil {
		t.Fatal(e)
	}
	recipe := uuid.New()
	if e = database.Exec("INSERT INTO recipes(id,user_id,delicacy_id,title,algo) VALUES (?,?,?,?,?)", recipe, owner, source.ID, "Test recipe", "Cook").Error; e != nil {
		t.Fatal(e)
	}
	if e = s.Merge(t.Context(), source.ID, uuid.New(), staff, "bad target"); e == nil {
		t.Fatal("invalid merge succeeded")
	}
	var linkedRaw string
	database.Raw("SELECT delicacy_id FROM recipes WHERE id=?", recipe).Scan(&linkedRaw)
	linked, _ := uuid.Parse(linkedRaw)
	if linked != source.ID {
		t.Fatal("failed merge did not roll back")
	}
	if e = s.Merge(t.Context(), source.ID, pending.ID, staff, "same canonical dish"); e != nil {
		t.Fatal(e)
	}
	database.Raw("SELECT delicacy_id FROM recipes WHERE id=?", recipe).Scan(&linkedRaw)
	linked, _ = uuid.Parse(linkedRaw)
	if linked != pending.ID {
		t.Fatal("recipe not moved")
	}
	redirected, e := s.Get(t.Context(), source.ID)
	if e != nil || redirected.ID != pending.ID {
		t.Fatalf("redirect=%#v error=%v", redirected, e)
	}
	assertAuditCount(t, database, source.ID, "dish.merged", 1)

	request.Name = "Withdraw me"
	request.ConfirmSimilar = true
	w, e := s.Create(t.Context(), request, owner, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Withdraw(t.Context(), w.ID, owner); e != nil {
		t.Fatal(e)
	}
	request.Name = "Reject me"
	r, e := s.Create(t.Context(), request, owner, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Moderate(t.Context(), r.ID, staff, domain.DelicacyRejected, "duplicate concept"); e != nil {
		t.Fatal(e)
	}
	assertAuditCount(t, database, r.ID, "dish.rejected", 1)
}

func assertAuditCount(t *testing.T, database *gorm.DB, id uuid.UUID, action string, want int64) {
	t.Helper()
	var got int64
	database.Raw("SELECT count(*) FROM audit_logs WHERE target_id=? AND action=?", id, action).Scan(&got)
	if got != want {
		t.Fatalf("%s audit count=%d", action, got)
	}
}
func dishTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("COOKED_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("COOKED_TEST_DATABASE_URL is not configured")
	}
	base, e := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "cooked_dish_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if e = base.Exec("CREATE SCHEMA " + schema).Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := base.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error; e != nil {
			t.Error(e)
		}
	})
	u, e := url.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
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
