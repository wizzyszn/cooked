package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"go.uber.org/zap"
	"gorm.io/datatypes"
)

type dispatcher struct {
	store     Store
	users     UserDirectory
	providers map[domain.NotificationChannel]ChannelProvider
	log       *zap.SugaredLogger
}

func newDispatcher(store Store, users UserDirectory, providers []ChannelProvider, log *zap.SugaredLogger) *dispatcher {
	indexed := make(map[domain.NotificationChannel]ChannelProvider, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		indexed[provider.Channel()] = provider
	}
	return &dispatcher{
		store:     store,
		users:     users,
		providers: indexed,
		log:       log,
	}
}

func (d *dispatcher) persist(ctx context.Context, req NotificationRequest) (*domain.Notification, error) {
	if req.Channel == "" {
		req.Channel = domain.NotificationChannelEmail
	}
	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal notification payload: %w", err)
	}
	if req.Payload == nil {
		payload = []byte("{}")
	}
	n := &domain.Notification{
		UserID:      req.UserID,
		Channel:     req.Channel,
		Template:    req.Template,
		PayloadJSON: datatypes.JSON(payload),
		Status:      domain.NotificationStatusPending,
	}
	if err := d.store.Create(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (d *dispatcher) deliver(ctx context.Context, id uuid.UUID, req NotificationRequest) {
	if req.Channel == "" {
		req.Channel = domain.NotificationChannelEmail
	}

	account, err := d.users.FindByID(ctx, req.UserID)
	if err != nil {
		d.fail(ctx, id, "lookup user", err)
		return
	}
	if account == nil {
		d.fail(ctx, id, "lookup user", fmt.Errorf("user %s not found", req.UserID))
		return
	}
	if account.Email == "" {
		if err := d.store.MarkSuppressed(ctx, id); err != nil && d.log != nil {
			d.log.Errorw("mark notification suppressed", "id", id, "error", err)
		}
		return
	}

	rendered := Render(req.Template, req.Payload)
	if rendered == nil {
		d.fail(ctx, id, "render template", fmt.Errorf("unknown template %q", req.Template))
		return
	}

	provider, ok := d.providers[req.Channel]
	if !ok {
		d.fail(ctx, id, "resolve provider", fmt.Errorf("no provider for channel %q", req.Channel))
		return
	}

	ref, err := provider.Send(ctx, Outbound{
		Name:     account.Name,
		Email:    account.Email,
		UserID:   account.ID.String(),
		Template: req.Template,
		Title:    rendered.Title,
		Body:     rendered.Body,
		HTML:     rendered.HTML,
		Payload:  req.Payload,
	})
	if err != nil {
		d.fail(ctx, id, "send", err)
		return
	}
	if err := d.store.MarkSent(ctx, id, ref); err != nil && d.log != nil {
		d.log.Errorw("mark notification sent", "id", id, "error", err)
	}
}

func (d *dispatcher) fail(ctx context.Context, id uuid.UUID, step string, err error) {
	if d.log != nil {
		d.log.Errorw("notification delivery failed", "id", id, "step", step, "error", err)
	}
	if markErr := d.store.MarkFailed(ctx, id); markErr != nil && d.log != nil {
		d.log.Errorw("mark notification failed", "id", id, "error", markErr)
	}
}

func requestFromNotification(n domain.Notification) (NotificationRequest, error) {
	var payload map[string]any
	if len(n.PayloadJSON) > 0 {
		if err := json.Unmarshal(n.PayloadJSON, &payload); err != nil {
			return NotificationRequest{}, fmt.Errorf("decode payload: %w", err)
		}
	}
	return NotificationRequest{
		UserID:   n.UserID,
		Channel:  n.Channel,
		Template: n.Template,
		Payload:  payload,
	}, nil
}
