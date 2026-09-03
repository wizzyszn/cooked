package engagement

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/config"
	appdb "github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/notify"
	"github.com/wizzyszn/cooked/internal/platform"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTrendReconciliationWeightsDeduplicatesAndAgesOut(t *testing.T) {
	database := engagementDB(t)
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	author := engagementUser(t, database, "trend-author")
	cook := engagementUser(t, database, "trend-cook")
	recipe, version := engagementRecipe(t, database, author, "public")
	for range 2 {
		mustEngage(t, database, "INSERT INTO cook_sessions(user_id,recipe_id,recipe_version_id,status,completed_at,completion_local_date,completion_timezone) VALUES (?,?,?,'completed',?,'2026-09-03','UTC')", cook, recipe, version, now)
	}
	mustEngage(t, database, "INSERT INTO favorites(user_id,recipe_id,created_at) VALUES (?,?,?)", cook, recipe, now)
	mustEngage(t, database, "INSERT INTO reviews(user_id,recipe_id,recipe_version_id,taste,clarity,difficulty_accuracy,moderation_status,created_at,updated_at) VALUES (?,?,?,5,4,3,'visible',?,?)", cook, recipe, version, now, now)
	privateRecipe, privateVersion := engagementRecipe(t, database, author, "private")
	mustEngage(t, database, "INSERT INTO favorites(user_id,recipe_id,created_at) VALUES (?,?,?)", cook, privateRecipe, now)
	_ = privateVersion
	p := NewTrendProjector(database, config.EngagementConfig{TrendCookWeight: 3, TrendFavoriteWeight: 1, TrendReviewWeight: 2, TrendWindowDays: 7})
	p.clock = platform.FixedClock{Time: now}
	if err := p.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
	var score, cooks int
	database.Raw("SELECT score,unique_cooks FROM recipe_trend_scores WHERE recipe_id=?", recipe).Row().Scan(&score, &cooks)
	if score != 6 || cooks != 1 {
		t.Fatalf("score=%d cooks=%d", score, cooks)
	}
	var publicCount int64
	database.Raw("SELECT count(*) FROM recipe_trend_scores t JOIN recipes r ON r.id=t.recipe_id WHERE r.visibility='public'").Scan(&publicCount)
	if publicCount != 1 {
		t.Fatalf("public trends=%d", publicCount)
	}
	secondSaver := engagementUser(t, database, "trend-second-saver")
	mustEngage(t, database, "INSERT INTO favorites(user_id,recipe_id,created_at) VALUES (?,?,?)", secondSaver, recipe, now)
	p.lastFull = now
	if err := p.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	database.Raw("SELECT score FROM recipe_trend_scores WHERE recipe_id=?", recipe).Scan(&score)
	if score != 7 {
		t.Fatalf("incremental score=%d", score)
	}
	p.clock = platform.FixedClock{Time: now.AddDate(0, 0, 8)}
	if err := p.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
	var rows int64
	database.Table("recipe_trend_scores").Count(&rows)
	if rows != 0 {
		t.Fatalf("stale trends=%d", rows)
	}
}

func TestPreferenceDefaultsAndReminderIdempotency(t *testing.T) {
	database := engagementDB(t)
	user := engagementUser(t, database, "streak-user")
	svc := notify.NewService(database)
	prefs, err := svc.Preferences(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range prefs {
		want := p.Channel == domain.NotificationChannelInApp
		if p.Enabled != want {
			t.Fatalf("default %#v", p)
		}
	}
	now := time.Date(2026, 9, 3, 19, 30, 0, 0, time.UTC)
	mustEngage(t, database, "UPDATE users SET current_streak=3,streak_last_qualifying_date='2026-09-02',timezone='UTC' WHERE id=?", user)
	w := NewReminderWorker(database, 19)
	w.clock = platform.FixedClock{Time: now}
	if err = w.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = w.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	var inapp, email int64
	database.Raw("SELECT count(*) FROM notifications WHERE user_id=? AND template='streak_at_risk' AND channel='in_app'", user).Scan(&inapp)
	database.Raw("SELECT count(*) FROM notifications WHERE user_id=? AND template='streak_at_risk' AND channel='email'", user).Scan(&email)
	if inapp != 1 || email != 0 {
		t.Fatalf("default reminders inapp=%d email=%d", inapp, email)
	}
	if err = svc.SetPreference(t.Context(), user, notify.PreferenceRequest{Category: "streak", Channel: domain.NotificationChannelEmail, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	mustEngage(t, database, "UPDATE users SET streak_last_qualifying_date='2026-09-03' WHERE id=?", user)
	w.clock = platform.FixedClock{Time: now.AddDate(0, 0, 1)}
	if err = w.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = w.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	database.Raw("SELECT count(*) FROM notifications WHERE user_id=? AND template='streak_at_risk' AND channel='in_app'", user).Scan(&inapp)
	database.Raw("SELECT count(*) FROM notifications WHERE user_id=? AND template='streak_at_risk' AND channel='email'", user).Scan(&email)
	if inapp != 2 || email != 1 {
		t.Fatalf("opt-in reminders inapp=%d email=%d", inapp, email)
	}
	inbox, err := svc.Inbox(t.Context(), user)
	if err != nil || inbox.UnreadCount != 2 {
		t.Fatalf("inbox=%#v err=%v", inbox, err)
	}
	if err = svc.MarkRead(t.Context(), user, inbox.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err = svc.SetPreference(t.Context(), user, notify.PreferenceRequest{Category: "activity", Channel: domain.NotificationChannelEmail, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	transactional := notify.NewOutboxNotifier(notify.NewStore(database), nil)
	if err = transactional.Notify(t.Context(), notify.NotificationRequest{UserID: user, Channel: domain.NotificationChannelEmail, Template: notify.TemplateVerifyEmail, Payload: map[string]any{"verify_url": "https://example.test"}}); err != nil {
		t.Fatal(err)
	}
	var authMail int64
	database.Raw("SELECT count(*) FROM notifications WHERE user_id=? AND category='transactional' AND template='verify_email'", user).Scan(&authMail)
	if authMail != 1 {
		t.Fatalf("transactional mail suppressed: %d", authMail)
	}
}

func engagementDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("COOKED_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("COOKED_TEST_DATABASE_URL is not configured")
	}
	base, e := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "cooked_engagement_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
func engagementUser(t *testing.T, db *gorm.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	mustEngage(t, db, "INSERT INTO users(id,email,name,user_name,is_verified,hash_pass) VALUES (?,?,?,?,true,'hash')", id, name+"@engage.test", name, name)
	return id
}
func engagementRecipe(t *testing.T, db *gorm.DB, author uuid.UUID, visibility string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	dish, recipe, version := uuid.New(), uuid.New(), uuid.New()
	mustEngage(t, db, "INSERT INTO delicacies(id,name,description,status,published_at) VALUES (?,?,'dish','published',now())", dish, "Dish "+dish.String())
	mustEngage(t, db, "INSERT INTO recipes(id,user_id,delicacy_id,title,algo,visibility,moderation_status) VALUES (?,?,?,'Recipe','',?,'visible')", recipe, author, dish, visibility)
	mustEngage(t, db, "INSERT INTO recipe_versions(id,recipe_id,version_number,lifecycle,title,published_at) VALUES (?,?,1,'published','Recipe',now())", version, recipe)
	mustEngage(t, db, "UPDATE recipes SET current_published_version_id=? WHERE id=?", version, recipe)
	return recipe, version
}
func mustEngage(t *testing.T, db *gorm.DB, q string, args ...any) {
	t.Helper()
	if e := db.Exec(q, args...).Error; e != nil {
		t.Fatal(e)
	}
}
