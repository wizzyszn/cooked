package notify

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	defaultQueueSize = 256
	defaultWorkers   = 2
	deliverTimeout   = 20 * time.Second
)

type queuedJob struct {
	id  uuid.UUID
	req NotificationRequest
}

// AsyncNotifier persists the notification immediately, then delivers it on a
// worker pool so HTTP handlers are not blocked on the email provider.
type AsyncNotifier struct {
	disp    *dispatcher
	queue   chan queuedJob
	workers int
	log     *zap.SugaredLogger

	mu     sync.RWMutex
	closed bool
	wg     sync.WaitGroup
}

func NewAsyncNotifier(store Store, users UserDirectory, providers []ChannelProvider, log *zap.SugaredLogger) *AsyncNotifier {
	n := &AsyncNotifier{
		disp:    newDispatcher(store, users, providers, log),
		queue:   make(chan queuedJob, defaultQueueSize),
		workers: defaultWorkers,
		log:     log,
	}
	n.start()
	n.recoverPending(context.Background())
	return n
}

func (n *AsyncNotifier) start() {
	for i := 0; i < n.workers; i++ {
		n.wg.Add(1)
		go n.loop()
	}
}

func (n *AsyncNotifier) loop() {
	defer n.wg.Done()
	for job := range n.queue {
		ctx, cancel := context.WithTimeout(context.Background(), deliverTimeout)
		n.disp.deliver(ctx, job.id, job.req)
		cancel()
	}
}

func (n *AsyncNotifier) Notify(ctx context.Context, req NotificationRequest) error {
	row, err := n.disp.persist(ctx, req)
	if err != nil {
		return err
	}

	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.closed {
		if n.log != nil {
			n.log.Warnw("notifier stopped; notification left pending", "id", row.ID)
		}
		return nil
	}

	job := queuedJob{id: row.ID, req: req}
	select {
	case n.queue <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *AsyncNotifier) recoverPending(ctx context.Context) {
	pending, err := n.disp.store.ListPending(ctx, defaultQueueSize)
	if err != nil {
		if n.log != nil {
			n.log.Errorw("recover pending notifications", "error", err)
		}
		return
	}
	for i := range pending {
		req, err := requestFromNotification(pending[i])
		if err != nil {
			if n.log != nil {
				n.log.Errorw("decode pending notification", "id", pending[i].ID, "error", err)
			}
			continue
		}
		select {
		case n.queue <- queuedJob{id: pending[i].ID, req: req}:
		default:
			if n.log != nil {
				n.log.Warnw("notification queue full during recover", "left", len(pending)-i)
			}
			return
		}
	}
}

func (n *AsyncNotifier) Stop() {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	n.closed = true
	close(n.queue)
	n.mu.Unlock()
	n.wg.Wait()
}
