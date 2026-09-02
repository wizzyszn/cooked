package middlewares

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

type IpEntry struct {
	tokens      float64
	lastChecked time.Time
}

type RateLimter struct {
	mu       sync.Mutex
	visitors map[string]*IpEntry
	rate     float64
	burst    int
}

func NewRateLimiter(requestPerMinute int) *RateLimter {
	rl := &RateLimter{
		visitors: make(map[string]*IpEntry),
		rate:     float64(requestPerMinute) / 60.0,
		burst:    requestPerMinute,
	}

	go rl.cleanUp()

	return rl

}

func (rl *RateLimter) cleanUp() {
	for {
		time.Sleep(time.Minute * 5)
		rl.mu.Lock()
		for ip, entry := range rl.visitors {
			if ip != "" {
				if time.Since(entry.lastChecked) > 5*time.Minute {
					delete(rl.visitors, ip)
				}
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	entry, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &IpEntry{
			tokens:      float64(rl.burst) - 1,
			lastChecked: time.Now(),
		}
		return true
	}

	elapsed := time.Since(entry.lastChecked).Seconds()
	entry.tokens += elapsed * rl.rate

	if entry.tokens > float64(rl.burst) {
		entry.tokens = float64(rl.burst)
	}
	entry.lastChecked = time.Now()

	if entry.tokens < 1 {
		return false
	}

	entry.tokens--

	return true
}
func (rl *RateLimter) Limit(ctx *gin.Context) {
	ip := ctx.ClientIP()
	if !rl.allow(ip) {
		models.WriteAppError(ctx, apperrors.ErrTooManyRequests)
		return
	}
}
