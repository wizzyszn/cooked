package platform

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	MinIdempotencyKeyLength = 8
	MaxIdempotencyKeyLength = 128
)

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

func ParseIdempotencyKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if len(key) < MinIdempotencyKeyLength || len(key) > MaxIdempotencyKeyLength {
		return "", fmt.Errorf("idempotency key must be between %d and %d characters", MinIdempotencyKeyLength, MaxIdempotencyKeyLength)
	}
	if !idempotencyKeyPattern.MatchString(key) {
		return "", fmt.Errorf("idempotency key contains unsupported characters")
	}
	return key, nil
}
