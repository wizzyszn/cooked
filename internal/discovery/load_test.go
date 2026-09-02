package discovery

import (
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestSearchLoadProfile is opt-in because the FRD requires a five-minute run.
// It creates 50,000 public recipes in the test's disposable schema and records
// the measured p95 plus query plan and environment details in test output.
func TestSearchLoadProfile(t *testing.T) {
	if os.Getenv("COOKED_RUN_M5_LOAD_TEST") != "1" {
		t.Skip("set COOKED_RUN_M5_LOAD_TEST=1")
	}
	database := discoveryDB(t)
	database.Logger = logger.Default.LogMode(logger.Silent)
	seedLoadDataset(t, database, 50000)
	assertIndexedTitlePlan(t, database)
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(60)
	sqlDB.SetMaxIdleConns(60)
	duration := 5 * time.Minute
	if raw := os.Getenv("COOKED_M5_LOAD_DURATION"); raw != "" {
		duration, err = time.ParseDuration(raw)
		if err != nil {
			t.Fatal(err)
		}
	}
	svc := NewService(NewRepository(database))
	deadline := time.Now().Add(duration)
	var wg sync.WaitGroup
	var failures atomic.Uint64
	latencies := make(chan time.Duration, 1000000)
	for worker := 0; worker < 50; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				f := Filters{Query: "jollof", Limit: 20}
				switch worker % 4 {
				case 1:
					f.Difficulty = "easy"
				case 2:
					f.Category = "load"
				case 3:
					f.Region = "load-region"
					f.MaxSeconds = intPtr(3600)
				}
				started := time.Now()
				_, e := svc.Search(t.Context(), f)
				elapsed := time.Since(started)
				if e != nil {
					failures.Add(1)
				}
				select {
				case latencies <- elapsed:
				default:
				}
			}
		}(worker)
	}
	wg.Wait()
	close(latencies)
	samples := make([]time.Duration, 0, len(latencies))
	for x := range latencies {
		samples = append(samples, x)
	}
	if len(samples) == 0 {
		t.Fatal("no latency samples")
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 := samples[(len(samples)*95-1)/100]
	var dbSize int64
	database.Raw("SELECT pg_database_size(current_database())").Scan(&dbSize)
	t.Logf("M5 load profile: recipes=50000 clients=50 duration=%s requests=%d failures=%d p95=%s go=%s cpu=%d database_bytes=%d", duration, len(samples), failures.Load(), p95, runtime.Version(), runtime.NumCPU(), dbSize)
	if failures.Load() != 0 {
		t.Fatalf("search failures=%d", failures.Load())
	}
	if p95 >= 300*time.Millisecond {
		t.Fatalf("NFR-1 unmet: p95=%s target=<300ms", p95)
	}
}

func seedLoadDataset(t *testing.T, database *gorm.DB, count int) {
	t.Helper()
	user, category, region, dish := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	must(t, database, "INSERT INTO users(id,email,name,user_name,is_verified,hash_pass) VALUES (?,'load@test.local','Load','load',true,'hash')", user)
	must(t, database, "INSERT INTO categories(id,name,slug) VALUES (?,'Load','load')", category)
	must(t, database, "INSERT INTO regions(id,name,slug) VALUES (?,'Load Region','load-region')", region)
	must(t, database, "INSERT INTO delicacies(id,name,description,category_id,status,published_at) VALUES (?,'Load Jollof','load',?,'published',now())", dish, category)
	must(t, database, "INSERT INTO delicacy_regions VALUES (?,?)", dish, region)
	must(t, database, `INSERT INTO recipes(id,user_id,delicacy_id,title,algo,visibility,moderation_status,created_at,updated_at)
	 SELECT gen_random_uuid(),?,?,'Load recipe '||n,'','public','visible',now(),now() FROM generate_series(1,?) n`, user, dish, count)
	must(t, database, `INSERT INTO recipe_versions(id,recipe_id,version_number,lifecycle,title,summary,base_servings,prep_time_seconds,cook_time_seconds,difficulty,published_at)
	 SELECT gen_random_uuid(),r.id,1,'published',CASE WHEN n%1000=0 THEN 'Jollof special '||n ELSE 'Weeknight recipe '||n END,'load',4,600,1200,CASE WHEN n%3=0 THEN 'medium' ELSE 'easy' END,now()-(n||' seconds')::interval
	 FROM (SELECT r.id,row_number() OVER (ORDER BY r.id) n FROM recipes r WHERE r.user_id=?) r`, user)
	must(t, database, "UPDATE recipes r SET current_published_version_id=v.id FROM recipe_versions v WHERE v.recipe_id=r.id AND r.user_id=?", user)
	must(t, database, "VACUUM recipe_versions")
	must(t, database, "ANALYZE recipes")
	must(t, database, "ANALYZE recipe_versions")
	must(t, database, "ANALYZE delicacies")
}
func assertIndexedTitlePlan(t *testing.T, database *gorm.DB) {
	t.Helper()
	var lines []string
	err := database.Raw("EXPLAIN SELECT id FROM recipe_versions WHERE lifecycle='published' AND lower(title) LIKE '%jollof%'").Scan(&lines).Error
	if err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(lines, "\n")
	t.Logf("title search plan:\n%s", plan)
	if !strings.Contains(plan, "idx_recipe_versions_title_trgm") {
		t.Fatalf("trigram index absent from plan: %s", plan)
	}
}
