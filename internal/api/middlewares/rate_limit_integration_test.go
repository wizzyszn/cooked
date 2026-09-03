package middlewares

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appdb "github.com/wizzyszn/cooked/internal/db"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSharedRateLimiterPersistsAcrossInstances(t *testing.T) {
	database := rateLimitDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NewSharedRateLimiter(database, "integration", 1, false).Limit)
	r.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	first := httptest.NewRecorder()
	r.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/limited", nil))
	secondRouter := gin.New()
	secondRouter.Use(NewSharedRateLimiter(database, "integration", 1, false).Limit)
	secondRouter.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	second := httptest.NewRecorder()
	secondRouter.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/limited", nil))
	if first.Code != http.StatusNoContent || second.Code != http.StatusTooManyRequests {
		t.Fatalf("statuses=%d,%d", first.Code, second.Code)
	}
}

func TestRateLimitCleanerOnlyRemovesExpiredBuckets(t *testing.T) {
	database := rateLimitDB(t)
	database.Exec(`INSERT INTO rate_limit_buckets(policy,subject_type,subject_key,window_started_at,request_count,expires_at) VALUES
		('expired','network','old',now()-interval '2 minutes',1,now()-interval '1 minute'),
		('active','network','current',now(),1,now()+interval '1 minute')`)
	if err := NewRateLimitCleaner(database).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	var expired, active int64
	database.Model(&struct{ Policy string }{}).Table("rate_limit_buckets").Where("policy='expired'").Count(&expired)
	database.Model(&struct{ Policy string }{}).Table("rate_limit_buckets").Where("policy='active'").Count(&active)
	if expired != 0 || active != 1 {
		t.Fatalf("expired=%d active=%d", expired, active)
	}
}
func rateLimitDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("COOKED_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("COOKED_TEST_DATABASE_URL is not configured")
	}
	base, e := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "cooked_limit_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
