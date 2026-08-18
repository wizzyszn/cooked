package notify

import (
	"context"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"go.uber.org/zap"
)

type UserDirectory interface {
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type NotificationRequest struct {
	UserID   uuid.UUID
	Channel  domain.NotificationChannel
	Template string
	Payload  map[string]any
}

type Notifier interface {
	Notify(ctx context.Context, req NotificationRequest) error
}

// SyncNotifier persists the notification and delivers it on the caller goroutine.
type SyncNotifier struct {
	disp *dispatcher
}

func NewSyncNotifier(store Store, users UserDirectory, providers []ChannelProvider, log *zap.SugaredLogger) *SyncNotifier {
	return &SyncNotifier{disp: newDispatcher(store, users, providers, log)}
}

func (n *SyncNotifier) Notify(ctx context.Context, req NotificationRequest) error {
	row, err := n.disp.persist(ctx, req)
	if err != nil {
		return err
	}
	n.disp.deliver(ctx, row.ID, req)
	return nil
}
