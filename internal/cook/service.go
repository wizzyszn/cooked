package cook

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/platform"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Rewards struct{ Base, Photo, FirstDish, DailySessions int }

func DefaultRewards() Rewards { return Rewards{Base: 50, Photo: 10, FirstDish: 25, DailySessions: 5} }

type Service struct {
	repo    *Repository
	clock   platform.Clock
	rewards Rewards
}

func NewService(repo *Repository) *Service {
	return NewServiceWithClock(repo, platform.RealClock{}, DefaultRewards())
}
func NewServiceWithClock(repo *Repository, clock platform.Clock, rewards Rewards) *Service {
	return &Service{repo: repo, clock: clock, rewards: rewards}
}

func (s *Service) Start(ctx context.Context, userID, versionID uuid.UUID) (*SessionView, error) {
	var recipeRaw string
	err := s.repo.db.WithContext(ctx).Raw(`SELECT r.id::text FROM recipe_versions v JOIN recipes r ON r.id=v.recipe_id WHERE v.id=? AND v.lifecycle='published' AND r.current_published_version_id IS NOT NULL AND r.deleted_at IS NULL AND r.moderation_status='visible' AND (r.visibility IN ('public','unlisted') OR r.user_id=?)`, versionID, userID).Scan(&recipeRaw).Error
	if err != nil {
		return nil, err
	}
	if recipeRaw == "" {
		return nil, apperrors.ErrNotFound
	}
	recipeID := uuid.MustParse(recipeRaw)
	now := s.clock.Now().UTC()
	id := uuid.New()
	err = s.repo.db.WithContext(ctx).Exec(`INSERT INTO cook_sessions(id,user_id,recipe_id,recipe_version_id,status,started_at,last_activity_at,created_at,updated_at) VALUES (?,?,?,?,'in_progress',?,?,?,?) ON CONFLICT (user_id,recipe_version_id) WHERE status='in_progress' DO NOTHING`, id, userID, recipeID, versionID, now, now, now, now).Error
	if err != nil {
		return nil, err
	}
	var actualRaw string
	if err = s.repo.db.WithContext(ctx).Raw("SELECT id::text FROM cook_sessions WHERE user_id=? AND recipe_version_id=? AND status='in_progress'", userID, versionID).Scan(&actualRaw).Error; err != nil {
		return nil, err
	}
	actual := uuid.MustParse(actualRaw)
	key := "cook-mode:" + actual.String()
	if err = s.serverEvent(ctx, s.repo.db, userID, "cook_mode_entered", &actual, &recipeID, &versionID, key, nil); err != nil {
		return nil, err
	}
	return s.Session(ctx, userID, actual)
}
func (s *Service) Active(ctx context.Context, userID, versionID uuid.UUID) (*SessionView, error) {
	var idRaw string
	err := s.repo.db.WithContext(ctx).Raw("SELECT id::text FROM cook_sessions WHERE user_id=? AND recipe_version_id=? AND status='in_progress'", userID, versionID).Scan(&idRaw).Error
	if err != nil {
		return nil, err
	}
	if idRaw == "" {
		return nil, apperrors.ErrNotFound
	}
	id := uuid.MustParse(idRaw)
	return s.Session(ctx, userID, id)
}
func (s *Service) Session(ctx context.Context, userID, id uuid.UUID) (*SessionView, error) {
	var session domain.CookSession
	err := s.repo.db.WithContext(ctx).First(&session, "id=? AND user_id=?", id, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	view := &SessionView{CookSession: session, VisitedStepIDs: []uuid.UUID{}, Timers: []domain.CookTimer{}}
	if err = s.repo.db.WithContext(ctx).Raw("SELECT recipe_step_id FROM cook_session_steps WHERE cook_session_id=? ORDER BY visited_at", id).Scan(&view.VisitedStepIDs).Error; err != nil {
		return nil, err
	}
	if err = s.repo.db.WithContext(ctx).Where("cook_session_id=?", id).Order("updated_at,id").Find(&view.Timers).Error; err != nil {
		return nil, err
	}
	now := s.clock.Now().UTC()
	for i := range view.Timers {
		if view.Timers[i].State == "running" && view.Timers[i].TargetAt != nil {
			view.Timers[i].RemainingSeconds = max(0, int(math.Ceil(view.Timers[i].TargetAt.Sub(now).Seconds())))
		}
	}
	return view, nil
}
func (s *Service) Visit(ctx context.Context, userID, sessionID, stepID uuid.UUID) error {
	now := s.clock.Now().UTC()
	result := s.repo.db.WithContext(ctx).Exec(`INSERT INTO cook_session_steps(cook_session_id,recipe_step_id,visited_at) SELECT cs.id,rs.id,? FROM cook_sessions cs JOIN recipe_version_steps rs ON rs.recipe_version_id=cs.recipe_version_id AND rs.id=? AND rs.deleted_at IS NULL WHERE cs.id=? AND cs.user_id=? AND cs.status='in_progress' ON CONFLICT DO NOTHING`, now, stepID, sessionID, userID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var ok int64
		s.repo.db.WithContext(ctx).Raw("SELECT count(*) FROM cook_session_steps WHERE cook_session_id=? AND recipe_step_id=?", sessionID, stepID).Scan(&ok)
		if ok == 0 {
			return apperrors.ErrNotFound
		}
	}
	s.repo.db.WithContext(ctx).Exec("UPDATE cook_sessions SET last_activity_at=?,updated_at=? WHERE id=?", now, now, sessionID)
	return s.serverEvent(ctx, s.repo.db, userID, "step_visited", &sessionID, nil, nil, "step:"+sessionID.String()+":"+stepID.String(), map[string]any{"step_id": stepID})
}
func (s *Service) Timer(ctx context.Context, userID, sessionID, stepID uuid.UUID, req TimerRequest) (*domain.CookTimer, error) {
	now := s.clock.Now().UTC()
	var out domain.CookTimer
	err := db.WithinTransaction(ctx, s.repo.db, func(tx *gorm.DB) error {
		var valid int64
		if e := tx.Raw(`SELECT count(*) FROM cook_sessions cs JOIN recipe_version_steps rs ON rs.recipe_version_id=cs.recipe_version_id WHERE cs.id=? AND cs.user_id=? AND cs.status='in_progress' AND rs.id=? AND rs.deleted_at IS NULL`, sessionID, userID, stepID).Scan(&valid).Error; e != nil {
			return e
		}
		if valid == 0 {
			return apperrors.ErrNotFound
		}
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("cook_session_id=? AND recipe_step_id=?", sessionID, stepID).First(&out).Error
		exists := e == nil
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		duration := 0
		if req.DurationSeconds != nil {
			duration = *req.DurationSeconds
		} else if exists {
			duration = out.DurationSeconds
		} else {
			tx.Raw("SELECT coalesce(duration_seconds,0) FROM recipe_version_steps WHERE id=?", stepID).Scan(&duration)
		}
		if duration <= 0 {
			return apperrors.ErrValidation
		}
		remaining := duration
		if exists {
			remaining = out.RemainingSeconds
			if out.State == "running" && out.TargetAt != nil {
				remaining = max(0, int(math.Ceil(out.TargetAt.Sub(now).Seconds())))
			}
		}
		values := map[string]any{"duration_seconds": duration, "updated_at": now}
		switch req.Action {
		case "start", "reset":
			values["state"] = "paused"
			values["remaining_seconds"] = duration
			values["target_at"] = nil
			values["started_at"] = nil
			if req.Action == "start" {
				values["state"] = "running"
				values["target_at"] = now.Add(time.Duration(duration) * time.Second)
				values["started_at"] = now
			}
		case "pause":
			if !exists || out.State != "running" {
				return apperrors.ErrConflict
			}
			values["state"] = "paused"
			values["remaining_seconds"] = remaining
			values["target_at"] = nil
		case "resume":
			if !exists || out.State != "paused" || remaining <= 0 {
				return apperrors.ErrConflict
			}
			values["state"] = "running"
			values["remaining_seconds"] = remaining
			values["target_at"] = now.Add(time.Duration(remaining) * time.Second)
			if out.StartedAt == nil {
				values["started_at"] = now
			}
		default:
			return apperrors.ErrValidation
		}
		if exists {
			if e = tx.Model(&domain.CookTimer{}).Where("id=?", out.ID).Updates(values).Error; e != nil {
				return e
			}
		} else {
			out = domain.CookTimer{ID: uuid.New(), CookSessionID: sessionID, RecipeStepID: stepID}
			values["id"] = out.ID
			values["cook_session_id"] = sessionID
			values["recipe_step_id"] = stepID
			if e = tx.Table("cook_timers").Create(values).Error; e != nil {
				return e
			}
		}
		tx.Exec("UPDATE cook_sessions SET last_activity_at=?,updated_at=? WHERE id=?", now, now, sessionID)
		return tx.First(&out, "id=?", out.ID).Error
	})
	if err != nil {
		return nil, err
	}
	_ = s.serverEvent(ctx, s.repo.db, userID, "timer_used", &sessionID, nil, nil, "timer:"+out.ID.String()+":"+now.Format(time.RFC3339Nano), map[string]any{"action": req.Action, "step_id": stepID})
	return &out, nil
}
func (s *Service) Abandon(ctx context.Context, userID, id uuid.UUID) error {
	now := s.clock.Now().UTC()
	return db.WithinTransaction(ctx, s.repo.db, func(tx *gorm.DB) error {
		result := tx.Model(&domain.CookSession{}).Where("id=? AND user_id=? AND status='in_progress'", id, userID).Updates(map[string]any{"status": "abandoned", "abandoned_at": now, "last_activity_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return apperrors.ErrNotFound
		}
		return s.serverEvent(ctx, tx, userID, "session_abandoned", &id, nil, nil, "abandon:"+id.String(), nil)
	})
}

func (s *Service) Complete(ctx context.Context, userID, id uuid.UUID, key string, photo *uuid.UUID) (*SessionView, error) {
	if _, err := platform.ParseIdempotencyKey(key); err != nil {
		return nil, apperrors.ErrValidation
	}
	now := s.clock.Now().UTC()
	err := db.WithinTransaction(ctx, s.repo.db, func(tx *gorm.DB) error {
		var account struct {
			Timezone                     string
			XPTotal                      int64
			CurrentStreak, LongestStreak int
			LastDate                     *time.Time
		}
		if e := tx.Raw("SELECT timezone,xp_total,current_streak,longest_streak,streak_last_qualifying_date last_date FROM users WHERE id=? AND deactivated_at IS NULL FOR UPDATE", userID).Scan(&account).Error; e != nil {
			return e
		}
		if account.Timezone == "" {
			return apperrors.ErrUnauthorized
		}
		var existingRaw string
		if e := tx.Raw("SELECT cook_session_id::text FROM cook_completion_commands WHERE user_id=? AND idempotency_key=?", userID, key).Scan(&existingRaw).Error; e != nil {
			return e
		}
		if existingRaw != "" {
			existing := uuid.MustParse(existingRaw)
			if existing != id {
				return apperrors.ErrConflict
			}
			return nil
		}
		var session domain.CookSession
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, "id=? AND user_id=?", id, userID).Error; e != nil {
			return apperrors.ErrNotFound
		}
		if session.Status != domain.CookSessionInProgress {
			return apperrors.ErrConflict
		}
		var total, visited int64
		tx.Raw("SELECT count(*) FROM recipe_version_steps WHERE recipe_version_id=? AND deleted_at IS NULL", session.RecipeVersionID).Scan(&total)
		tx.Raw("SELECT count(*) FROM cook_session_steps WHERE cook_session_id=?", id).Scan(&visited)
		if total == 0 || visited != total {
			return apperrors.New("STEPS_INCOMPLETE", "every recipe step must be visited", http.StatusConflict)
		}
		if photo != nil {
			var valid int64
			tx.Raw("SELECT count(*) FROM media_assets WHERE id=? AND owner_id=? AND purpose='cook_session_photo' AND processing_status='ready' AND moderation_status='approved' AND deleted_at IS NULL", *photo, userID).Scan(&valid)
			if valid == 0 {
				return apperrors.New("MEDIA_NOT_READY", "completion photo must be owned, processed, and approved", http.StatusConflict)
			}
		}
		location, e := time.LoadLocation(account.Timezone)
		if e != nil {
			return e
		}
		localNow := now.In(location)
		localDate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
		var rewarded, sameRecipe, firstDish int64
		tx.Raw("SELECT count(*) FROM cook_sessions WHERE user_id=? AND status='completed' AND completion_local_date=? AND xp_awarded>0", userID, localDate).Scan(&rewarded)
		tx.Raw("SELECT count(*) FROM cook_sessions WHERE user_id=? AND recipe_id=? AND status='completed' AND completion_local_date=?", userID, session.RecipeID, localDate).Scan(&sameRecipe)
		tx.Raw(`SELECT count(*) FROM cook_sessions cs JOIN recipes r ON r.id=cs.recipe_id JOIN recipes current ON current.id=? WHERE cs.user_id=? AND cs.status='completed' AND r.delicacy_id=current.delicacy_id`, session.RecipeID, userID).Scan(&firstDish)
		capped := rewarded >= int64(s.rewards.DailySessions)
		base, photoXP, dishXP := 0, 0, 0
		if !capped && sameRecipe == 0 {
			base = s.rewards.Base
			if photo != nil {
				photoXP = s.rewards.Photo
			}
			if firstDish == 0 {
				dishXP = s.rewards.FirstDish
			}
		}
		totalXP := base + photoXP + dishXP
		if e = tx.Model(&domain.CookSession{}).Where("id=?", id).Updates(map[string]any{"status": "completed", "photo_media_id": photo, "completed_at": now, "last_activity_at": now, "completion_local_date": localDate, "completion_timezone": account.Timezone, "xp_awarded": totalXP, "updated_at": now}).Error; e != nil {
			return e
		}
		decisions := []struct {
			kind     string
			amount   int
			decision string
		}{{"base", base, choice(base > 0, "awarded", "not_awarded")}, {"photo_bonus", photoXP, choice(photo == nil, "no_photo", choice(photoXP > 0, "awarded", "capped"))}, {"first_dish_bonus", dishXP, choice(dishXP > 0, "awarded", "already_completed")}}
		if capped {
			decisions = append(decisions, struct {
				kind     string
				amount   int
				decision string
			}{"daily_session_cap", 0, "daily_cap_reached"})
		}
		if sameRecipe > 0 {
			decisions = append(decisions, struct {
				kind     string
				amount   int
				decision string
			}{"recipe_day_cap", 0, "recipe_already_rewarded"})
		}
		for _, d := range decisions {
			if e = tx.Exec("INSERT INTO xp_ledger_entries(user_id,cook_session_id,local_date,kind,amount,decision,idempotency_key) VALUES (?,?,?,?,?,?,?)", userID, id, localDate, d.kind, d.amount, d.decision, key).Error; e != nil {
				return e
			}
		}
		previous, next, decision := account.CurrentStreak, 1, "started"
		if account.LastDate != nil {
			days := int(localDate.Sub(*account.LastDate).Hours() / 24)
			if days == 0 {
				next = previous
				decision = "same_day"
			} else if days == 1 {
				next = previous + 1
				decision = "advanced"
			}
		}
		longest := max(account.LongestStreak, next)
		if e = tx.Exec("INSERT INTO streak_ledger_entries(user_id,cook_session_id,local_date,previous_streak,new_streak,decision,idempotency_key) VALUES (?,?,?,?,?,?,?)", userID, id, localDate, previous, next, decision, key).Error; e != nil {
			return e
		}
		if e = tx.Exec("UPDATE users SET xp_total=xp_total+?,current_streak=?,longest_streak=?,streak_last_qualifying_date=?,updated_at=? WHERE id=?", totalXP, next, longest, localDate, now, userID).Error; e != nil {
			return e
		}
		if e = tx.Exec("INSERT INTO cook_completion_commands(user_id,idempotency_key,cook_session_id) VALUES (?,?,?)", userID, key, id).Error; e != nil {
			return e
		}
		if e = s.serverEvent(ctx, tx, userID, "session_completed", &id, &session.RecipeID, &session.RecipeVersionID, key, map[string]any{"xp_awarded": totalXP, "local_date": localDate.Format("2006-01-02")}); e != nil {
			return e
		}
		var prior int64
		tx.Raw("SELECT count(*) FROM cook_sessions WHERE user_id=? AND status='completed' AND id<>?", userID, id).Scan(&prior)
		if prior == 0 {
			return s.serverEvent(ctx, tx, userID, "user_activated", &id, &session.RecipeID, &session.RecipeVersionID, key, nil)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Session(ctx, userID, id)
}
func choice(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}

var clientEvents = map[string]map[string]bool{
	"search_impression": {"result_count": true, "has_filters": true},
	"browse_impression": {"result_count": true, "grouping": true},
	"recipe_viewed":     {"source": true},
	"favorite_toggled":  {"saved": true},
	"cook_mode_viewed":  {"entrypoint": true},
}

func (s *Service) Ingest(ctx context.Context, userID *uuid.UUID, req EventRequest) error {
	req.EventName = normalizeEventName(req.EventName)
	allowed, exists := clientEvents[req.EventName]
	if !exists || (userID == nil && req.AnonymousID == nil) {
		return apperrors.ErrValidation
	}
	for key, value := range req.Properties {
		if !allowed[key] || !safeAnalyticsValue(value) {
			return apperrors.ErrValidation
		}
	}
	payload, e := json.Marshal(req.Properties)
	if e != nil {
		return apperrors.ErrValidation
	}
	event := domain.AnalyticsEvent{UserID: userID, AnonymousID: req.AnonymousID, EventName: req.EventName, SchemaVersion: 1, Source: "client", RecipeID: req.RecipeID, RecipeVersionID: req.RecipeVersionID, Properties: datatypes.JSON(payload), OccurredAt: s.clock.Now().UTC()}
	return s.repo.db.WithContext(ctx).Create(&event).Error
}
func safeAnalyticsValue(value any) bool {
	switch v := value.(type) {
	case nil, bool, float64:
		return true
	case string:
		return len(v) <= 64
	default:
		return false
	}
}
func (s *Service) serverEvent(ctx context.Context, tx *gorm.DB, userID uuid.UUID, name string, session, recipe, version *uuid.UUID, key string, properties map[string]any) error {
	payload, _ := json.Marshal(properties)
	if payload == nil {
		payload = []byte("{}")
	}
	event := domain.AnalyticsEvent{UserID: &userID, EventName: name, SchemaVersion: 1, Source: "server", CookSessionID: session, RecipeID: recipe, RecipeVersionID: version, IdempotencyKey: &key, Properties: datatypes.JSON(payload), OccurredAt: s.clock.Now().UTC()}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&event).Error
}
func (s *Service) List(ctx context.Context, userID uuid.UUID, cursor string, limit int) (SessionPage, error) {
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 {
		return SessionPage{}, apperrors.ErrValidation
	}
	q := s.repo.db.WithContext(ctx).Where("user_id=?", userID)
	if cursor != "" {
		c, e := platform.DecodeCursor(cursor)
		if e != nil {
			return SessionPage{}, apperrors.ErrValidation
		}
		q = q.Where("(started_at,id)<(?,?)", c.Timestamp, c.ID)
	}
	var rows []domain.CookSession
	if e := q.Order("started_at DESC,id DESC").Limit(limit + 1).Find(&rows).Error; e != nil {
		return SessionPage{}, e
	}
	out := SessionPage{Items: []SessionView{}}
	for _, row := range rows[:min(len(rows), limit)] {
		v, e := s.Session(ctx, userID, row.ID)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, *v)
	}
	if len(rows) > limit {
		last := rows[limit-1]
		out.NextCursor, _ = platform.EncodeCursor(platform.Cursor{Timestamp: last.StartedAt, ID: last.ID})
	}
	return out, nil
}
func (s *Service) Metrics(ctx context.Context) (Metrics, error) {
	var m Metrics
	now := s.clock.Now().UTC()
	m.GeneratedAt = now
	q := s.repo.db.WithContext(ctx)
	if e := q.Raw("SELECT count(DISTINCT user_id) FROM cook_sessions WHERE status='completed'").Scan(&m.ActivationCount).Error; e != nil {
		return m, e
	}
	q.Raw("SELECT count(*) FROM analytics_events WHERE event_name='cook_mode_entered'").Scan(&m.CookModeEntries)
	q.Raw("SELECT count(*) FROM cook_sessions WHERE status='completed'").Scan(&m.CompletedSessions)
	m.ReviewEligibleCompletions = m.CompletedSessions
	if m.CookModeEntries > 0 {
		m.CookModeConversion = float64(m.CompletedSessions) / float64(m.CookModeEntries)
	}
	if err := q.Raw(`WITH firsts AS (SELECT user_id,min(completed_at) activated_at FROM cook_sessions WHERE status='completed' GROUP BY user_id), matured AS (SELECT * FROM firsts WHERE activated_at<=?), returned AS (SELECT DISTINCT m.user_id FROM matured m JOIN cook_sessions cs ON cs.user_id=m.user_id AND cs.status='completed' AND cs.completed_at>m.activated_at AND cs.completed_at<=m.activated_at+interval '7 days') SELECT (SELECT count(*) FROM matured) matured,(SELECT count(*) FROM returned) returned`, now.AddDate(0, 0, -7)).Row().Scan(&m.ActivatedCohortsMatured, &m.SevenDayReturners); err != nil {
		return m, err
	}
	if m.ActivatedCohortsMatured > 0 {
		m.SevenDayRetention = float64(m.SevenDayReturners) / float64(m.ActivatedCohortsMatured)
	}
	return m, nil
}

type StreakProjector struct {
	db    *gorm.DB
	clock platform.Clock
}

func NewStreakProjector(database *gorm.DB) *StreakProjector {
	return &StreakProjector{db: database, clock: platform.RealClock{}}
}
func (p *StreakProjector) RunOnce(ctx context.Context) error {
	now := p.clock.Now().UTC()
	return p.db.WithContext(ctx).Exec("UPDATE users SET current_streak=0,updated_at=? WHERE current_streak>0 AND streak_last_qualifying_date < ((? AT TIME ZONE timezone)::date - 1)", now, now).Error
}

func normalizeEventName(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
