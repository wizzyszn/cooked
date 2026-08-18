package notify

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
)

type memStore struct {
	mu    sync.Mutex
	items map[uuid.UUID]*domain.Notification
}

func newMemStore() *memStore {
	return &memStore{items: map[uuid.UUID]*domain.Notification{}}
}

func (s *memStore) Create(_ context.Context, n *domain.Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	cp := *n
	s.items[n.ID] = &cp
	return nil
}

func (s *memStore) MarkSent(_ context.Context, id uuid.UUID, ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.items[id]; ok {
		n.Status = domain.NotificationStatusSent
		n.ExternalRef = &ref
	}
	return nil
}

func (s *memStore) MarkFailed(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.items[id]; ok {
		n.Status = domain.NotificationStatusFailed
	}
	return nil
}

func (s *memStore) MarkSuppressed(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.items[id]; ok {
		n.Status = domain.NotificationStatusSuppressed
	}
	return nil
}

func (s *memStore) ListPending(_ context.Context, _ int) ([]domain.Notification, error) {
	return nil, nil
}

type staticUsers struct {
	user *domain.User
}

func (u staticUsers) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if u.user == nil || u.user.ID != id {
		return nil, nil
	}
	return u.user, nil
}

type captureProvider struct {
	got Outbound
}

func (p *captureProvider) Channel() domain.NotificationChannel {
	return domain.NotificationChannelEmail
}

func (p *captureProvider) Send(_ context.Context, msg Outbound) (string, error) {
	p.got = msg
	return "brevo-test-id", nil
}

func TestSyncNotifierSendsVerifyEmail(t *testing.T) {
	account := &domain.User{
		BaseModel: domain.BaseModel{ID: uuid.New()},
		Email:     "ada@example.com",
		Name:      "Ada",
	}
	store := newMemStore()
	provider := &captureProvider{}
	n := NewSyncNotifier(store, staticUsers{user: account}, []ChannelProvider{provider}, nil)

	if err := n.Notify(context.Background(), NotificationRequest{
		UserID:   account.ID,
		Channel:  domain.NotificationChannelEmail,
		Template: TemplateVerifyEmail,
		Payload: map[string]any{
			"name":       "Ada",
			"verify_url": "https://example.test/verify",
		},
	}); err != nil {
		t.Fatalf("notify: %v", err)
	}

	if provider.got.Email != account.Email {
		t.Fatalf("sent to %q", provider.got.Email)
	}
	if provider.got.Title == "" || provider.got.HTML == "" {
		t.Fatalf("empty outbound: %+v", provider.got)
	}
	if len(store.items) != 1 {
		t.Fatalf("expected 1 stored notification, got %d", len(store.items))
	}
	for _, row := range store.items {
		if row.Status != domain.NotificationStatusSent {
			t.Fatalf("status %s", row.Status)
		}
		if row.ExternalRef == nil || *row.ExternalRef != "brevo-test-id" {
			t.Fatalf("external ref %+v", row.ExternalRef)
		}
	}
}
