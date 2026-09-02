package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	brevo "github.com/getbrevo/brevo-go/lib"
	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/domain"
	"go.uber.org/zap"
)

type BrevoEmailProvider struct {
	client      *brevo.APIClient
	senderEmail string
	senderName  string
	log         *zap.SugaredLogger
}

func NewBrevoEmailProvider(cfg *config.BrevoConfig, log *zap.SugaredLogger) *BrevoEmailProvider {
	apiCfg := brevo.NewConfiguration()
	apiCfg.AddDefaultHeader("api-key", cfg.APIKey)
	if cfg.BaseUrl != "" {
		apiCfg.BasePath = strings.TrimRight(cfg.BaseUrl, "/")
	}
	apiCfg.HTTPClient = &http.Client{
		Timeout: 15 * time.Second,
	}
	return &BrevoEmailProvider{
		client:      brevo.NewAPIClient(apiCfg),
		senderEmail: cfg.SenderEmail,
		senderName:  cfg.SenderName,
		log:         log,
	}
}

func (p *BrevoEmailProvider) Channel() domain.NotificationChannel {
	return domain.NotificationChannelEmail
}

func (p *BrevoEmailProvider) Send(ctx context.Context, msg Outbound) (string, error) {
	if msg.Email == "" {
		return "", fmt.Errorf("no email on file")
	}

	html := msg.HTML
	if html == "" {
		html = fmt.Sprintf(
			`<html><body><p>%s</p></body></html>`,
			htmlEscape(msg.Body),
		)
	}

	email := brevo.SendSmtpEmail{
		Sender: &brevo.SendSmtpEmailSender{
			Name:  p.senderName,
			Email: p.senderEmail,
		},
		To: []brevo.SendSmtpEmailTo{
			{
				Email: msg.Email,
				Name:  msg.Name,
			},
		},
		HtmlContent: html,
		Subject:     msg.Title,
		TextContent: msg.Body,
		Tags:        []string{"cooked", msg.Template},
		Headers:     map[string]interface{}{"idempotencyKey": msg.IdempotencyKey},
	}

	result, resp, err := p.client.TransactionalEmailsApi.SendTransacEmail(ctx, email)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		detail := err.Error()
		if resp != nil && resp.Body != nil {
			if b, rerr := io.ReadAll(io.LimitReader(resp.Body, 1<<20)); rerr == nil && len(b) > 0 {
				detail = fmt.Sprintf("%s: %s", err.Error(), strings.TrimSpace(string(b)))
			}
		}
		return "", fmt.Errorf("brevo send: %s", detail)
	}

	ref := result.MessageId
	if ref == "" && len(result.MessageIds) > 0 {
		ref = result.MessageIds[0]
	}
	if ref == "" {
		ref = fmt.Sprintf("brevo-%d", time.Now().UnixNano())
	}
	if p.log != nil {
		p.log.Infow("brevo email sent",
			"user_id", msg.UserID,
			"template", msg.Template,
			"email", msg.Email,
			"message_id", ref,
		)
	}
	return ref, nil
}
