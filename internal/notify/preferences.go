package notify

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var optionalCategories = map[string]bool{"activity": true, "streak": true}

type Preference struct {
	Category string                     `json:"category"`
	Channel  domain.NotificationChannel `json:"channel"`
	Enabled  bool                       `json:"enabled"`
}
type PreferenceRequest struct {
	Category string                     `json:"category"`
	Channel  domain.NotificationChannel `json:"channel"`
	Enabled  bool                       `json:"enabled"`
}
type Inbox struct {
	Items       []domain.Notification `json:"items"`
	UnreadCount int64                 `json:"unread_count"`
}

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func defaultEnabled(channel domain.NotificationChannel) bool {
	return channel == domain.NotificationChannelInApp
}

func (s *Service) Preferences(ctx context.Context, user uuid.UUID) ([]Preference, error) {
	out := make([]Preference, 0, 4)
	for _, category := range []string{"activity", "streak"} {
		for _, channel := range []domain.NotificationChannel{domain.NotificationChannelInApp, domain.NotificationChannelEmail} {
			p := Preference{Category: category, Channel: channel, Enabled: defaultEnabled(channel)}
			var explicit *bool
			if err := s.db.WithContext(ctx).Raw("SELECT enabled FROM notification_preferences WHERE user_id=? AND category=? AND channel=?", user, category, channel).Scan(&explicit).Error; err != nil {
				return nil, err
			}
			if explicit != nil {
				p.Enabled = *explicit
			}
			out = append(out, p)
		}
	}
	return out, nil
}
func (s *Service) SetPreference(ctx context.Context, user uuid.UUID, q PreferenceRequest) error {
	if !optionalCategories[q.Category] || (q.Channel != domain.NotificationChannelInApp && q.Channel != domain.NotificationChannelEmail) {
		return apperrors.ErrValidation
	}
	return s.db.WithContext(ctx).Exec("INSERT INTO notification_preferences(user_id,category,channel,enabled,updated_at) VALUES (?,?,?,?,now()) ON CONFLICT(user_id,category,channel) DO UPDATE SET enabled=excluded.enabled,updated_at=excluded.updated_at", user, q.Category, q.Channel, q.Enabled).Error
}
func (s *Service) Inbox(ctx context.Context, user uuid.UUID) (Inbox, error) {
	out := Inbox{Items: []domain.Notification{}}
	q := s.db.WithContext(ctx).Where("user_id=? AND channel='in_app' AND deleted_at IS NULL", user)
	if err := q.Order("created_at DESC,id DESC").Limit(50).Find(&out.Items).Error; err != nil {
		return out, err
	}
	if err := q.Where("read_at IS NULL").Count(&out.UnreadCount).Error; err != nil {
		return out, err
	}
	return out, nil
}
func (s *Service) MarkRead(ctx context.Context, user, id uuid.UUID) error {
	res := s.db.WithContext(ctx).Model(&domain.Notification{}).Where("id=? AND user_id=? AND channel='in_app'", id, user).Update("read_at", time.Now().UTC())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func PersistOptional(ctx context.Context, tx *gorm.DB, user uuid.UUID, category, template, key string, payload map[string]any) error {
	if !optionalCategories[category] {
		return apperrors.ErrValidation
	}
	b, _ := json.Marshal(payload)
	for _, channel := range []domain.NotificationChannel{domain.NotificationChannelInApp, domain.NotificationChannelEmail} {
		var enabled *bool
		if err := tx.WithContext(ctx).Raw("SELECT enabled FROM notification_preferences WHERE user_id=? AND category=? AND channel=?", user, category, channel).Scan(&enabled).Error; err != nil {
			return err
		}
		if enabled == nil && !defaultEnabled(channel) || enabled != nil && !*enabled {
			continue
		}
		intent := strings.Join([]string{key, string(channel)}, ":")
		status := domain.NotificationStatusPending
		if channel == domain.NotificationChannelInApp {
			status = domain.NotificationStatusSent
		}
		n := domain.Notification{UserID: user, Channel: channel, Category: category, Template: template, PayloadJSON: datatypes.JSON(b), Status: status, NextAttemptAt: time.Now().UTC(), IdempotencyKey: &intent}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&n).Error; err != nil {
			return err
		}
	}
	return nil
}
