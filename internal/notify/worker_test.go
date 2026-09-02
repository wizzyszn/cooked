package notify

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"gorm.io/datatypes"
)

type memoryWorkerStore struct {
	mu             sync.Mutex
	row            domain.Notification
	failFinishOnce bool
	attempts       int
}

func (s *memoryWorkerStore) ClaimPending(context.Context, string, int, time.Time) ([]domain.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.row.Status == domain.NotificationStatusSent || s.row.AttemptCount >= 5 {
		return nil, nil
	}
	row := s.row
	s.row.AttemptCount++
	return []domain.Notification{row}, nil
}
func (s *memoryWorkerStore) StartAttempt(context.Context, uuid.UUID, int, string, time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts++
	return nil
}
func (s *memoryWorkerStore) FinishSent(_ context.Context, _ uuid.UUID, _ int, _ string, ref string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failFinishOnce {
		s.failFinishOnce = false
		return errors.New("database unavailable after provider accepted message")
	}
	s.row.Status = domain.NotificationStatusSent
	s.row.ExternalRef = &ref
	return nil
}
func (s *memoryWorkerStore) FinishSuppressed(context.Context, uuid.UUID, int, string, time.Time) error {
	return nil
}
func (s *memoryWorkerStore) FinishFailed(context.Context, uuid.UUID, int, string, string, bool, time.Time, time.Time) error {
	return nil
}

type idempotentProvider struct {
	mu       sync.Mutex
	accepted map[string]string
	calls    int
}

func (p *idempotentProvider) Channel() domain.NotificationChannel {
	return domain.NotificationChannelEmail
}
func (p *idempotentProvider) Send(_ context.Context, msg Outbound) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if ref := p.accepted[msg.IdempotencyKey]; ref != "" {
		return ref, nil
	}
	ref := "provider-message-1"
	p.accepted[msg.IdempotencyKey] = ref
	return ref, nil
}

func TestWorkerRestartReusesProviderIdempotencyKey(t *testing.T) {
	userID := uuid.New()
	store := &memoryWorkerStore{failFinishOnce: true, row: domain.Notification{BaseModel: domain.BaseModel{ID: uuid.New()}, UserID: userID, Channel: domain.NotificationChannelEmail, Template: TemplateVerifyEmail, PayloadJSON: datatypes.JSON([]byte(`{"name":"Ada","verify_url":"https://example.test"}`)), Status: domain.NotificationStatusPending}}
	provider := &idempotentProvider{accepted: map[string]string{}}
	users := staticUsers{user: &domain.User{BaseModel: domain.BaseModel{ID: userID}, Name: "Ada", Email: "ada@example.test"}}
	if err := NewWorker(store, users, []ChannelProvider{provider}, "worker-a", 1, nil).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := NewWorker(store, users, []ChannelProvider{provider}, "worker-b", 1, nil).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if store.row.Status != domain.NotificationStatusSent {
		t.Fatalf("status=%s", store.row.Status)
	}
	if len(provider.accepted) != 1 || provider.calls != 2 {
		t.Fatalf("provider accepted=%d calls=%d", len(provider.accepted), provider.calls)
	}
}
