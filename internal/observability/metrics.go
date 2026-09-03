package observability

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Registry struct {
	mu                  sync.Mutex
	requests            map[string]uint64
	errors              map[string]uint64
	latencyMS           map[string]uint64
	completionConflicts uint64
}

func NewRegistry() *Registry {
	return &Registry{requests: map[string]uint64{}, errors: map[string]uint64{}, latencyMS: map[string]uint64{}}
}
func (r *Registry) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		key := c.Request.Method + " " + route
		r.mu.Lock()
		r.requests[key]++
		r.latencyMS[key] += uint64(time.Since(start).Milliseconds())
		if c.Writer.Status() >= 400 {
			r.errors[key]++
		}
		if c.Writer.Status() == http.StatusConflict && c.FullPath() == "/api/v1/cook-sessions/:id/complete" {
			r.completionConflicts++
		}
		r.mu.Unlock()
	}
}
func (r *Registry) Handler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var b strings.Builder
		r.mu.Lock()
		for key, count := range r.requests {
			label := strings.ReplaceAll(key, "\"", "'")
			fmt.Fprintf(&b, "cooked_http_requests_total{route=\"%s\"} %d\n", label, count)
			fmt.Fprintf(&b, "cooked_http_request_latency_ms_sum{route=\"%s\"} %d\n", label, r.latencyMS[key])
			fmt.Fprintf(&b, "cooked_http_errors_total{route=\"%s\"} %d\n", label, r.errors[key])
		}
		fmt.Fprintf(&b, "cooked_completion_conflicts_total %d\n", r.completionConflicts)
		r.mu.Unlock()
		if db != nil {
			if sqlDB, err := db.DB(); err == nil {
				stats := sqlDB.Stats()
				fmt.Fprintf(&b, "cooked_db_open_connections %d\ncooked_db_in_use_connections %d\ncooked_db_idle_connections %d\n", stats.OpenConnections, stats.InUse, stats.Idle)
			}
			appendDBMetrics(db, &b)
		}
		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(b.String()))
	}
}
func appendDBMetrics(db *gorm.DB, b *strings.Builder) {
	queries := []struct{ name, sql string }{{"cooked_notification_queue_age_seconds", "SELECT COALESCE(extract(epoch FROM now()-min(created_at)),0) FROM notifications WHERE status IN ('pending','failed')"}, {"cooked_notification_job_retries_total", "SELECT COALESCE(sum(attempt_count),0) FROM notifications"}, {"cooked_provider_failures_total", "SELECT count(*) FROM notification_delivery_attempts WHERE status='failed'"}, {"cooked_notification_suppressions_total", "SELECT count(*) FROM notifications WHERE status='suppressed'"}, {"cooked_media_processing_failures_total", "SELECT count(*) FROM media_assets WHERE processing_status='failed'"}, {"cooked_media_processing_queue_age_seconds", "SELECT COALESCE(extract(epoch FROM now()-min(created_at)),0) FROM media_assets WHERE processing_status IN ('uploaded','retry','processing')"}}
	for _, q := range queries {
		var value float64
		if err := db.Raw(q.sql).Scan(&value).Error; err == nil {
			fmt.Fprintf(b, "%s %g\n", q.name, value)
		}
	}
}
