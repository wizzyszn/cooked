package middlewares

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
	"gorm.io/gorm"
)

type SharedRateLimiter struct {
	db      *gorm.DB
	policy  string
	limit   int
	window  time.Duration
	account bool
}

func NewSharedRateLimiter(db *gorm.DB, policy string, requestsPerMinute int, account bool) *SharedRateLimiter {
	return &SharedRateLimiter{db: db, policy: policy, limit: requestsPerMinute, window: time.Minute, account: account}
}
func (rl *SharedRateLimiter) Limit(c *gin.Context) {
	if rl == nil || rl.db == nil || rl.limit < 1 {
		models.WriteAppError(c, apperrors.ErrServiceUnavailable)
		return
	}
	if !rl.allow(c.Request.Context(), "network", c.ClientIP()) {
		models.WriteAppError(c, apperrors.ErrTooManyRequests)
		return
	}
	if rl.account {
		if user, ok := CurrentUserFromContext(c); ok && !rl.allow(c.Request.Context(), "account", user.ID.String()) {
			models.WriteAppError(c, apperrors.ErrTooManyRequests)
			return
		}
	}
	c.Next()
}
func (rl *SharedRateLimiter) allow(ctx context.Context, kind, key string) bool {
	now := time.Now().UTC()
	var count int
	err := rl.db.WithContext(ctx).Raw(`INSERT INTO rate_limit_buckets(policy,subject_type,subject_key,window_started_at,request_count,expires_at) VALUES (?,?,?,?,1,?) ON CONFLICT(policy,subject_type,subject_key) DO UPDATE SET request_count=CASE WHEN rate_limit_buckets.window_started_at<=? THEN 1 ELSE rate_limit_buckets.request_count+1 END,window_started_at=CASE WHEN rate_limit_buckets.window_started_at<=? THEN ? ELSE rate_limit_buckets.window_started_at END,expires_at=? RETURNING request_count`, rl.policy, kind, key, now, now.Add(rl.window), now.Add(-rl.window), now.Add(-rl.window), now, now.Add(rl.window)).Scan(&count).Error
	return err == nil && count <= rl.limit
}
func (rl *SharedRateLimiter) Cleanup(ctx context.Context) error {
	return rl.db.WithContext(ctx).Exec("DELETE FROM rate_limit_buckets WHERE expires_at<?", time.Now().UTC()).Error
}

// RateLimitCleaner removes logically expired shared buckets. Request handling
// never depends on cleanup timing, but pruning bounds storage as client keys
// change over time.
type RateLimitCleaner struct {
	db *gorm.DB
}

func NewRateLimitCleaner(db *gorm.DB) *RateLimitCleaner { return &RateLimitCleaner{db: db} }

func (c *RateLimitCleaner) RunOnce(ctx context.Context) error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.WithContext(ctx).Exec("DELETE FROM rate_limit_buckets WHERE expires_at < ?", time.Now().UTC()).Error
}
