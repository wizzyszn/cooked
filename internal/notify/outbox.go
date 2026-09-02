package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"go.uber.org/zap"
)

// OutboxNotifier only commits notification intent. cmd/worker owns delivery.
type OutboxNotifier struct{ disp *dispatcher }

func NewOutboxNotifier(store Store, log *zap.SugaredLogger) *OutboxNotifier {
	return &OutboxNotifier{disp: newDispatcher(store, nil, nil, log)}
}
func (n *OutboxNotifier) Notify(ctx context.Context, req NotificationRequest) error {
	_, err := n.disp.persist(ctx, req)
	return err
}

type WorkerStore interface {
	ClaimPending(context.Context, string, int, time.Time) ([]domain.Notification, error)
	StartAttempt(context.Context, uuid.UUID, int, string, time.Time) error
	FinishSent(context.Context, uuid.UUID, int, string, string, time.Time) error
	FinishSuppressed(context.Context, uuid.UUID, int, string, time.Time) error
	FinishFailed(context.Context, uuid.UUID, int, string, string, bool, time.Time, time.Time) error
}

type Worker struct {
	store     WorkerStore
	users     UserDirectory
	providers map[domain.NotificationChannel]ChannelProvider
	workerID  string
	batchSize int
	log       *zap.SugaredLogger
	now       func() time.Time
}

func NewWorker(store WorkerStore, users UserDirectory, providers []ChannelProvider, workerID string, batchSize int, log *zap.SugaredLogger) *Worker {
	indexed := map[domain.NotificationChannel]ChannelProvider{}
	for _, p := range providers {
		if p != nil {
			indexed[p.Channel()] = p
		}
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	return &Worker{store: store, users: users, providers: indexed, workerID: workerID, batchSize: batchSize, log: log, now: func() time.Time { return time.Now().UTC() }}
}
func (w *Worker) RunOnce(ctx context.Context) error {
	rows, err := w.store.ClaimPending(ctx, w.workerID, w.batchSize, w.now())
	if err != nil {
		return err
	}
	for i := range rows {
		w.deliver(ctx, &rows[i])
	}
	return nil
}
func (w *Worker) deliver(ctx context.Context, n *domain.Notification) {
	attempt := n.AttemptCount + 1
	providerKey := fmt.Sprintf("%s:%d", n.ID, attempt)
	if err := w.store.StartAttempt(ctx, n.ID, attempt, providerKey, w.now()); err != nil {
		w.fail(ctx, n, attempt, providerKey, err)
		return
	}
	req, err := requestFromNotification(*n)
	if err != nil {
		w.fail(ctx, n, attempt, providerKey, err)
		return
	}
	account, err := w.users.FindByID(ctx, n.UserID)
	if err != nil || account == nil {
		if err == nil {
			err = fmt.Errorf("user not found")
		}
		w.fail(ctx, n, attempt, providerKey, err)
		return
	}
	if account.Email == "" || account.DeactivatedAt != nil {
		_ = w.store.FinishSuppressed(ctx, n.ID, attempt, providerKey, w.now())
		return
	}
	rendered := Render(req.Template, req.Payload)
	if rendered == nil {
		w.fail(ctx, n, attempt, providerKey, fmt.Errorf("unknown template %q", req.Template))
		return
	}
	provider := w.providers[req.Channel]
	if provider == nil {
		w.fail(ctx, n, attempt, providerKey, fmt.Errorf("no provider for channel %q", req.Channel))
		return
	}
	ref, err := provider.Send(ctx, Outbound{Name: account.Name, Email: account.Email, UserID: account.ID.String(), Template: req.Template, Title: rendered.Title, Body: rendered.Body, HTML: rendered.HTML, Payload: req.Payload, IdempotencyKey: n.ID.String()})
	if err != nil {
		w.fail(ctx, n, attempt, providerKey, err)
		return
	}
	_ = w.store.FinishSent(ctx, n.ID, attempt, providerKey, ref, w.now())
}
func (w *Worker) fail(ctx context.Context, n *domain.Notification, attempt int, key string, err error) {
	retry := attempt < 5
	delay := time.Duration(1<<min(attempt-1, 6)) * time.Minute
	if e := w.store.FinishFailed(ctx, n.ID, attempt, key, err.Error(), retry, w.now().Add(delay), w.now()); e != nil && w.log != nil {
		w.log.Errorw("record notification failure", "notification_id", n.ID, "error", e)
	}
	if w.log != nil {
		w.log.Warnw("notification delivery failed", "notification_id", n.ID, "retry", retry, "error", err)
	}
}
