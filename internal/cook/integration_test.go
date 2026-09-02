package cook

import (
	"github.com/google/uuid"
	appdb "github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/platform"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type cookedRecipe struct {
	recipe, version uuid.UUID
	steps           []uuid.UUID
}

func TestSessionTimerCompletionAndIdempotency(t *testing.T) {
	database := cookDB(t)
	user := seedCookUser(t, database, "cook-one", "America/New_York")
	recipe := seedCookRecipe(t, database, user, uuid.New())
	clock := &mutableClock{now: time.Date(2026, 3, 8, 6, 55, 0, 0, time.UTC)}
	svc := NewServiceWithClock(NewRepository(database), clock, DefaultRewards())
	session, e := svc.Start(t.Context(), user, recipe.version)
	if e != nil {
		t.Fatal(e)
	}
	again, e := svc.Start(t.Context(), user, recipe.version)
	if e != nil || again.ID != session.ID {
		t.Fatalf("resume=%#v err=%v", again, e)
	}
	if e = svc.Visit(t.Context(), user, session.ID, recipe.steps[0]); e != nil {
		t.Fatal(e)
	}
	timer, e := svc.Timer(t.Context(), user, session.ID, recipe.steps[0], TimerRequest{Action: "start", DurationSeconds: intPointer(120)})
	if e != nil || timer.State != "running" {
		t.Fatalf("timer=%#v err=%v", timer, e)
	}
	clock.now = clock.now.Add(30 * time.Second)
	restarted := NewServiceWithClock(NewRepository(database), clock, DefaultRewards())
	restored, e := restarted.Session(t.Context(), user, session.ID)
	if e != nil || len(restored.Timers) != 1 || restored.Timers[0].RemainingSeconds != 90 {
		t.Fatalf("restored=%#v err=%v", restored, e)
	}
	if _, e = restarted.Complete(t.Context(), user, session.ID, "complete:one", nil); e == nil {
		t.Fatal("completed before every step visit")
	}
	if e = restarted.Visit(t.Context(), user, session.ID, recipe.steps[1]); e != nil {
		t.Fatal(e)
	}
	photo := uuid.New()
	mustCook(t, database, "INSERT INTO media_assets(id,owner_id,purpose,object_key,declared_mime_type,processing_status,moderation_status,access_scope,expires_at) VALUES (?,?,'cook_session_photo',?,'image/jpeg','ready','approved','private',?)", photo, user, "cook/"+photo.String(), clock.now.Add(time.Hour))
	completed, e := restarted.Complete(t.Context(), user, session.ID, "complete:one", &photo)
	if e != nil || completed.XPAwarded != 85 {
		t.Fatalf("complete=%#v err=%v", completed, e)
	}
	retry, e := restarted.Complete(t.Context(), user, session.ID, "complete:one", nil)
	if e != nil || retry.ID != completed.ID || retry.XPAwarded != 85 {
		t.Fatalf("retry=%#v err=%v", retry, e)
	}
	var xp, streak, events int64
	database.Raw("SELECT count(*) FROM xp_ledger_entries WHERE cook_session_id=?", session.ID).Scan(&xp)
	database.Raw("SELECT count(*) FROM streak_ledger_entries WHERE cook_session_id=?", session.ID).Scan(&streak)
	database.Raw("SELECT count(*) FROM analytics_events WHERE cook_session_id=? AND event_name IN ('session_completed','user_activated')", session.ID).Scan(&events)
	if xp != 3 || streak != 1 || events != 2 {
		t.Fatalf("xp=%d streak=%d events=%d", xp, streak, events)
	}
	var zone string
	database.Raw("SELECT completion_timezone FROM cook_sessions WHERE id=?", session.ID).Scan(&zone)
	if zone != "America/New_York" {
		t.Fatalf("timezone=%s", zone)
	}
	repeat, _ := restarted.Start(t.Context(), user, recipe.version)
	for _, step := range recipe.steps {
		restarted.Visit(t.Context(), user, repeat.ID, step)
	}
	repeated, e := restarted.Complete(t.Context(), user, repeat.ID, "complete:repeat", nil)
	if e != nil || repeated.XPAwarded != 0 {
		t.Fatalf("recipe/day cap=%#v err=%v", repeated, e)
	}
	sameDish := seedCookRecipe(t, database, user, recipeDish(t, database, recipe.recipe))
	other, _ := restarted.Start(t.Context(), user, sameDish.version)
	for _, step := range sameDish.steps {
		restarted.Visit(t.Context(), user, other.ID, step)
	}
	otherDone, e := restarted.Complete(t.Context(), user, other.ID, "complete:same-dish", nil)
	if e != nil || otherDone.XPAwarded != 50 {
		t.Fatalf("first-dish repeat=%#v err=%v", otherDone, e)
	}
}
func TestConcurrentCompletionHasOneRewardSet(t *testing.T) {
	database := cookDB(t)
	user := seedCookUser(t, database, "cook-race", "UTC")
	r := seedCookRecipe(t, database, user, uuid.New())
	svc := NewServiceWithClock(NewRepository(database), platform.FixedClock{Time: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}, DefaultRewards())
	session, _ := svc.Start(t.Context(), user, r.version)
	for _, step := range r.steps {
		if e := svc.Visit(t.Context(), user, session.ID, step); e != nil {
			t.Fatal(e)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := svc.Complete(t.Context(), user, session.ID, "complete:race", nil)
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var total, count int64
	database.Raw("SELECT coalesce(sum(amount),0),count(*) FROM xp_ledger_entries WHERE cook_session_id=?", session.ID).Row().Scan(&total, &count)
	if total != 75 || count != 3 {
		t.Fatalf("total=%d rows=%d", total, count)
	}
}
func TestRewardCapsStreakTimezoneAndMetrics(t *testing.T) {
	database := cookDB(t)
	user := seedCookUser(t, database, "cook-rules", "UTC")
	clock := &mutableClock{now: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)}
	svc := NewServiceWithClock(NewRepository(database), clock, DefaultRewards())
	var sessions []uuid.UUID
	for i := 0; i < 6; i++ {
		r := seedCookRecipe(t, database, user, uuid.New())
		session, _ := svc.Start(t.Context(), user, r.version)
		for _, step := range r.steps {
			svc.Visit(t.Context(), user, session.ID, step)
		}
		done, e := svc.Complete(t.Context(), user, session.ID, "cap:"+uuid.NewString(), nil)
		if e != nil {
			t.Fatal(e)
		}
		sessions = append(sessions, session.ID)
		if i < 5 && done.XPAwarded != 75 {
			t.Fatalf("session %d xp=%d", i, done.XPAwarded)
		}
		if i == 5 && done.XPAwarded != 0 {
			t.Fatalf("sixth xp=%d", done.XPAwarded)
		}
	}
	var capRows int64
	database.Raw("SELECT count(*) FROM xp_ledger_entries WHERE cook_session_id=? AND kind='daily_session_cap' AND amount=0", sessions[5]).Scan(&capRows)
	if capRows != 1 {
		t.Fatalf("daily cap ledger rows=%d", capRows)
	}
	clock.now = clock.now.Add(24 * time.Hour)
	r2 := seedCookRecipe(t, database, user, uuid.New())
	s2, _ := svc.Start(t.Context(), user, r2.version)
	for _, step := range r2.steps {
		svc.Visit(t.Context(), user, s2.ID, step)
	}
	done, e := svc.Complete(t.Context(), user, s2.ID, "next-day:"+uuid.NewString(), nil)
	if e != nil || done.XPAwarded != 75 {
		t.Fatalf("next day=%#v err=%v", done, e)
	}
	clock.now = clock.now.Add(48 * time.Hour)
	database.Exec("UPDATE users SET timezone='Africa/Lagos' WHERE id=?", user)
	r3 := seedCookRecipe(t, database, user, uuid.New())
	s3, _ := svc.Start(t.Context(), user, r3.version)
	for _, step := range r3.steps {
		svc.Visit(t.Context(), user, s3.ID, step)
	}
	if _, e = svc.Complete(t.Context(), user, s3.ID, "missed:"+uuid.NewString(), nil); e != nil {
		t.Fatal(e)
	}
	var current, longest int
	database.Raw("SELECT current_streak,longest_streak FROM users WHERE id=?", user).Row().Scan(&current, &longest)
	if current != 1 || longest != 2 {
		t.Fatalf("streak current=%d longest=%d", current, longest)
	}
	var firstZone string
	database.Raw("SELECT completion_timezone FROM cook_sessions WHERE id=?", sessions[0]).Scan(&firstZone)
	if firstZone != "UTC" {
		t.Fatalf("historical timezone changed: %s", firstZone)
	}
	metrics, e := svc.Metrics(t.Context())
	if e != nil || metrics.ActivationCount != 1 || metrics.CompletedSessions != 8 || metrics.SevenDayReturners != 0 {
		t.Fatalf("metrics=%#v err=%v", metrics, e)
	}
	clock.now = clock.now.Add(48 * time.Hour)
	projector := &StreakProjector{db: database, clock: clock}
	if e = projector.RunOnce(t.Context()); e != nil {
		t.Fatal(e)
	}
	database.Raw("SELECT current_streak FROM users WHERE id=?", user).Scan(&current)
	if current != 0 {
		t.Fatalf("expired streak projection=%d", current)
	}
}

func TestStreakUsesLocalDatesAcrossDST(t *testing.T) {
	database := cookDB(t)
	user := seedCookUser(t, database, "cook-dst", "America/New_York")
	clock := &mutableClock{now: time.Date(2026, 3, 8, 4, 30, 0, 0, time.UTC)}
	svc := NewServiceWithClock(NewRepository(database), clock, DefaultRewards())
	for i, instant := range []time.Time{time.Date(2026, 3, 8, 4, 30, 0, 0, time.UTC), time.Date(2026, 3, 9, 3, 30, 0, 0, time.UTC)} {
		clock.now = instant
		r := seedCookRecipe(t, database, user, uuid.New())
		session, _ := svc.Start(t.Context(), user, r.version)
		for _, step := range r.steps {
			svc.Visit(t.Context(), user, session.ID, step)
		}
		if _, e := svc.Complete(t.Context(), user, session.ID, "dst:"+string(rune('a'+i))+uuid.NewString(), nil); e != nil {
			t.Fatal(e)
		}
	}
	var current int
	database.Raw("SELECT current_streak FROM users WHERE id=?", user).Scan(&current)
	if current != 2 {
		t.Fatalf("DST streak=%d", current)
	}
}

func TestSevenDayRetentionReconcilesFromSessions(t *testing.T) {
	database := cookDB(t)
	user := seedCookUser(t, database, "cook-retention", "UTC")
	clock := &mutableClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	svc := NewServiceWithClock(NewRepository(database), clock, DefaultRewards())
	for i, day := range []int{1, 5} {
		clock.now = time.Date(2026, 1, day, 12, 0, 0, 0, time.UTC)
		r := seedCookRecipe(t, database, user, uuid.New())
		session, _ := svc.Start(t.Context(), user, r.version)
		for _, step := range r.steps {
			svc.Visit(t.Context(), user, session.ID, step)
		}
		if _, e := svc.Complete(t.Context(), user, session.ID, "retention:"+string(rune('a'+i))+uuid.NewString(), nil); e != nil {
			t.Fatal(e)
		}
	}
	clock.now = time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	metrics, e := svc.Metrics(t.Context())
	if e != nil || metrics.ActivatedCohortsMatured != 1 || metrics.SevenDayReturners != 1 || metrics.SevenDayRetention != 1 {
		t.Fatalf("metrics=%#v err=%v", metrics, e)
	}
}

type mutableClock struct{ now time.Time }

func (c *mutableClock) Now() time.Time { return c.now }
func intPointer(v int) *int            { return &v }
func seedCookUser(t *testing.T, db *gorm.DB, name, timezone string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	mustCook(t, db, "INSERT INTO users(id,email,name,user_name,is_verified,hash_pass,timezone) VALUES (?,?,?, ?,true,'hash',?)", id, name+"@test.local", name, name, timezone)
	return id
}
func seedCookRecipe(t *testing.T, db *gorm.DB, author, dish uuid.UUID) cookedRecipe {
	t.Helper()
	var exists int64
	db.Raw("SELECT count(*) FROM delicacies WHERE id=?", dish).Scan(&exists)
	if exists == 0 {
		mustCook(t, db, "INSERT INTO delicacies(id,name,description,status,published_at) VALUES (?,?,'test','published',now())", dish, "Dish "+dish.String())
	}
	r := cookedRecipe{recipe: uuid.New(), version: uuid.New(), steps: []uuid.UUID{uuid.New(), uuid.New()}}
	mustCook(t, db, "INSERT INTO recipes(id,user_id,delicacy_id,title,algo,visibility,moderation_status) VALUES (?,?,?,'Cook recipe','','public','visible')", r.recipe, author, dish)
	mustCook(t, db, "INSERT INTO recipe_versions(id,recipe_id,version_number,lifecycle,title,base_servings) VALUES (?,?,1,'draft','Cook recipe',2)", r.version, r.recipe)
	for i, step := range r.steps {
		mustCook(t, db, "INSERT INTO recipe_version_steps(id,recipe_version_id,position,title,instruction,action,duration_seconds) VALUES (?,?,?,'Step','Do it','other',120)", step, r.version, i)
	}
	mustCook(t, db, "UPDATE recipe_versions SET lifecycle='published',published_at=now() WHERE id=?", r.version)
	mustCook(t, db, "UPDATE recipes SET current_published_version_id=? WHERE id=?", r.version, r.recipe)
	return r
}
func mustCook(t *testing.T, db *gorm.DB, q string, args ...any) {
	t.Helper()
	if e := db.Exec(q, args...).Error; e != nil {
		t.Fatal(e)
	}
}
func recipeDish(t *testing.T, db *gorm.DB, recipe uuid.UUID) uuid.UUID {
	t.Helper()
	var raw string
	if e := db.Raw("SELECT delicacy_id::text FROM recipes WHERE id=?", recipe).Scan(&raw).Error; e != nil {
		t.Fatal(e)
	}
	return uuid.MustParse(raw)
}
func cookDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("COOKED_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("COOKED_TEST_DATABASE_URL is not configured")
	}
	base, e := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "cooked_cook_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
