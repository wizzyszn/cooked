package notify

import (
	"context"

	"github.com/wizzyszn/cooked/internal/domain"
)

type ChannelProvider interface {
	Channel() domain.NotificationChannel
	Send(ctx context.Context, msg Outbound) (externalRef string, err error)
}

type Outbound struct {
	Name     string
	Email    string
	UserID   string
	Template string
	Title    string
	Body     string
	HTML     string
	Payload  map[string]any
}
