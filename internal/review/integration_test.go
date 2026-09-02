package review

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	appdb "github.com/wizzyszn/cooked/internal/db"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestReviewEligibilityAggregatesAndIdempotency(t *testing.T) {
	database := reviewDB(t)
	author := seedUser(t, database, "author", true)
	reviewer := seedUser(t, database, "reviewer", true)
	unverified := seedUser(t, database, "unverified", false)
	recipeID, v1, v2 := seedVersions(t, database, author)
	complete(t, database, reviewer, recipeID, v1)
	svc := NewService(NewRepository(database))

	q := WriteRequest{Taste: 5, Clarity: 4, DifficultyAccuracy: 3, Comment: "Useful"}
	if _, err := svc.Create(t.Context(), unverified, v1, "review:unverified", q); err != apperrors.ErrEmailNotVerified {
		t.Fatalf("unverified error=%v", err)
	}
	if _, err := svc.Create(t.Context(), author, v1, "review:self", q); err != apperrors.ErrForbidden {
		t.Fatalf("author error=%v", err)
	}
	if _, err := svc.Create(t.Context(), reviewer, v2, "review:wrong-version", q); err == nil {
		t.Fatal("reviewed a version without completing it")
	}
	created, err := svc.Create(t.Context(), reviewer, v1, "review:create-one", q)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := svc.Create(t.Context(), reviewer, v1, "review:create-one", q)
	if err != nil || retry.ID != created.ID {
		t.Fatalf("retry=%#v err=%v", retry, err)
	}
	if _, err = svc.Create(t.Context(), reviewer, v1, "review:create-two", q); err == nil {
		t.Fatal("second review was accepted")
	}
	q.Taste = 1
	if _, err = svc.Edit(t.Context(), reviewer, created.ID, q); err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(t.Context(), v1, nil, false)
	if err != nil || list.Aggregate.ReviewCount != 1 || list.Aggregate.AverageTaste != 1 {
		t.Fatalf("list=%#v err=%v", list, err)
	}
	v2List, err := svc.List(t.Context(), v2, nil, false)
	if err != nil || v2List.Aggregate.ReviewCount != 0 {
		t.Fatalf("historical aggregate leaked: %#v err=%v", v2List, err)
	}
}

func TestThreeVerifiedReportsHideOnceAndModerationAudits(t *testing.T) {
	database := reviewDB(t)
	author := seedUser(t, database, "report-author", true)
	reviewer := seedUser(t, database, "report-reviewer", true)
	recipeID, versionID, _ := seedVersions(t, database, author)
	complete(t, database, reviewer, recipeID, versionID)
	svc := NewService(NewRepository(database))
	reviewRow, err := svc.Create(t.Context(), reviewer, versionID, "review:reported", WriteRequest{Taste: 4, Clarity: 4, DifficultyAccuracy: 4})
	if err != nil {
		t.Fatal(err)
	}
	unverified := seedUser(t, database, "report-unverified", false)
	q := ReportRequest{TargetType: "review", TargetID: reviewRow.ID, Reason: "spam"}
	if _, err = svc.Report(t.Context(), unverified, "report:unverified", q); err != apperrors.ErrEmailNotVerified {
		t.Fatalf("unverified report=%v", err)
	}
	reporters := []uuid.UUID{seedUser(t, database, "report-one", true), seedUser(t, database, "report-two", true), seedUser(t, database, "report-three", true)}
	var first *Report
	for i, reporter := range reporters {
		row, reportErr := svc.Report(t.Context(), reporter, "report:key-"+string(rune('a'+i)), q)
		if reportErr != nil {
			t.Fatal(reportErr)
		}
		if i == 0 {
			first = row
			if _, duplicateErr := svc.Report(t.Context(), reporter, "report:duplicate", q); duplicateErr == nil {
				t.Fatal("duplicate report accepted")
			}
		}
	}
	got, err := svc.Get(t.Context(), reviewRow.ID, nil, false)
	if err == nil || got != nil {
		t.Fatal("auto-hidden review remained public")
	}
	if _, err = svc.Report(t.Context(), seedUser(t, database, "report-four", true), "report:after-hide", q); err == nil {
		t.Fatal("report after hide accepted")
	}
	moderator := seedUser(t, database, "moderator", true)
	if err = svc.Moderate(t.Context(), moderator, first.ID, ModerationRequest{Action: "restore", Reason: "reports dismissed"}); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Get(t.Context(), reviewRow.ID, nil, false); err != nil {
		t.Fatal("restored review unavailable")
	}
	var audits, pending int64
	database.Raw("SELECT count(*) FROM audit_logs WHERE target_type='review' AND target_id=?", reviewRow.ID).Scan(&audits)
	database.Raw("SELECT count(*) FROM content_reports WHERE target_type='review' AND target_id=? AND state='pending'", reviewRow.ID).Scan(&pending)
	if audits != 2 || pending != 0 {
		t.Fatalf("audits=%d pending=%d", audits, pending)
	}
	var removalReport *Report
	for i := range 3 {
		reporter := seedUser(t, database, "remove-reporter-"+string(rune('a'+i)), true)
		row, reportErr := svc.Report(t.Context(), reporter, "report:remove-"+string(rune('a'+i)), q)
		if reportErr != nil {
			t.Fatal(reportErr)
		}
		if i == 0 {
			removalReport = row
		}
	}
	if err = svc.Moderate(t.Context(), moderator, removalReport.ID, ModerationRequest{Action: "remove", Reason: "confirmed violation"}); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Get(t.Context(), reviewRow.ID, &reviewer, false); err == nil {
		t.Fatal("removed review remained available to its author")
	}
	database.Raw("SELECT count(*) FROM audit_logs WHERE target_type='review' AND target_id=?", reviewRow.ID).Scan(&audits)
	if audits != 4 {
		t.Fatalf("audits after removal=%d", audits)
	}
}

func reviewDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("COOKED_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("COOKED_TEST_DATABASE_URL is not configured")
	}
	base, err := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "cooked_review_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { base.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE") })
	u, _ := url.Parse(raw)
	values := u.Query()
	values.Set("search_path", schema+",public")
	u.RawQuery = values.Encode()
	database, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = appdb.Migrate(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func seedUser(t *testing.T, db *gorm.DB, name string, verified bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec("INSERT INTO users(id,email,name,user_name,is_verified,hash_pass) VALUES (?,?,?,?,?,'hash')", id, name+"@review.test", name, name, verified).Error; err != nil {
		t.Fatal(err)
	}
	return id
}
func seedVersions(t *testing.T, db *gorm.DB, author uuid.UUID) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	dish, recipeID, v1, v2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := db.Exec("INSERT INTO delicacies(id,name,description,status,published_at) VALUES (?,?,?,'published',now())", dish, "Dish "+dish.String(), "dish").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO recipes(id,user_id,delicacy_id,title,algo,visibility,moderation_status) VALUES (?,?,?,'Recipe','','public','visible')", recipeID, author, dish).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO recipe_versions(id,recipe_id,version_number,lifecycle,title,published_at) VALUES (?,?,1,'published','V1',now()),(?,?,2,'published','V2',now())", v1, recipeID, v2, recipeID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE recipes SET current_published_version_id=? WHERE id=?", v2, recipeID).Error; err != nil {
		t.Fatal(err)
	}
	return recipeID, v1, v2
}
func complete(t *testing.T, db *gorm.DB, user, recipeID, versionID uuid.UUID) {
	t.Helper()
	if err := db.Exec("INSERT INTO cook_sessions(user_id,recipe_id,recipe_version_id,status,completed_at,completion_local_date,completion_timezone) VALUES (?,?,?,'completed',now(),current_date,'UTC')", user, recipeID, versionID).Error; err != nil {
		t.Fatal(err)
	}
}
