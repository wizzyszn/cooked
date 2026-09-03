package engagement

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/notify"
	"github.com/wizzyszn/cooked/internal/platform"
	"gorm.io/gorm"
)

type TrendProjector struct {
	db       *gorm.DB
	cfg      config.EngagementConfig
	clock    platform.Clock
	lastFull time.Time
}

func NewTrendProjector(db *gorm.DB, cfg config.EngagementConfig) *TrendProjector {
	return &TrendProjector{db: db, cfg: cfg, clock: platform.RealClock{}}
}
func (p *TrendProjector) RunOnce(ctx context.Context) error {
	now := p.clock.Now().UTC()
	if p.lastFull.IsZero() || now.Sub(p.lastFull) >= time.Hour {
		if err := p.Reconcile(ctx); err != nil {
			return err
		}
		p.lastFull = now
		return nil
	}
	return p.RefreshQueued(ctx)
}

func (p *TrendProjector) RefreshQueued(ctx context.Context) error {
	now := p.clock.Now().UTC()
	cutoff := now.AddDate(0, 0, -p.cfg.TrendWindowDays)
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rawIDs []string
		if err := tx.Raw("SELECT recipe_id::text FROM trend_projection_queue ORDER BY queued_at LIMIT 100 FOR UPDATE SKIP LOCKED").Scan(&rawIDs).Error; err != nil {
			return err
		}
		ids := make([]uuid.UUID, 0, len(rawIDs))
		for _, raw := range rawIDs {
			id, err := uuid.Parse(raw)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		for _, id := range ids {
			if err := p.refreshRecipe(tx, id, cutoff, now); err != nil {
				return err
			}
		}
		if len(ids) > 0 {
			return tx.Exec("DELETE FROM trend_projection_queue WHERE recipe_id IN ?", ids).Error
		}
		return nil
	})
}

func (p *TrendProjector) refreshRecipe(tx *gorm.DB, id uuid.UUID, cutoff, now time.Time) error {
	return tx.Exec(`INSERT INTO recipe_trend_scores(recipe_id,unique_cooks,new_favorites,new_reviews,score,window_started_at,computed_at) SELECT ?,cooks,favs,revs,cooks*?+favs*?+revs*?,?,? FROM (SELECT (SELECT count(*) FROM (SELECT DISTINCT user_id,completion_local_date FROM cook_sessions WHERE recipe_id=? AND status='completed' AND completed_at>=?) c) cooks,(SELECT count(*) FROM favorites WHERE recipe_id=? AND created_at>=?) favs,(SELECT count(*) FROM reviews WHERE recipe_id=? AND moderation_status='visible' AND created_at>=?) revs) x ON CONFLICT(recipe_id) DO UPDATE SET unique_cooks=excluded.unique_cooks,new_favorites=excluded.new_favorites,new_reviews=excluded.new_reviews,score=excluded.score,window_started_at=excluded.window_started_at,computed_at=excluded.computed_at`, id, p.cfg.TrendCookWeight, p.cfg.TrendFavoriteWeight, p.cfg.TrendReviewWeight, cutoff, now, id, cutoff, id, cutoff, id, cutoff).Error
}
func (p *TrendProjector) Reconcile(ctx context.Context) error {
	now := p.clock.Now().UTC()
	cutoff := now.AddDate(0, 0, -p.cfg.TrendWindowDays)
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`WITH cooks AS (SELECT recipe_id,count(*) n FROM (SELECT DISTINCT user_id,recipe_id,completion_local_date FROM cook_sessions WHERE status='completed' AND completed_at>=?) x GROUP BY recipe_id), favs AS (SELECT recipe_id,count(*) n FROM favorites WHERE created_at>=? GROUP BY recipe_id), revs AS (SELECT recipe_id,count(*) n FROM reviews WHERE created_at>=? AND moderation_status='visible' GROUP BY recipe_id), ids AS (SELECT recipe_id FROM cooks UNION SELECT recipe_id FROM favs UNION SELECT recipe_id FROM revs) INSERT INTO recipe_trend_scores(recipe_id,unique_cooks,new_favorites,new_reviews,score,window_started_at,computed_at) SELECT ids.recipe_id,COALESCE(c.n,0),COALESCE(f.n,0),COALESCE(v.n,0),COALESCE(c.n,0)*?+COALESCE(f.n,0)*?+COALESCE(v.n,0)*?,?,? FROM ids LEFT JOIN cooks c USING(recipe_id) LEFT JOIN favs f USING(recipe_id) LEFT JOIN revs v USING(recipe_id) ON CONFLICT(recipe_id) DO UPDATE SET unique_cooks=excluded.unique_cooks,new_favorites=excluded.new_favorites,new_reviews=excluded.new_reviews,score=excluded.score,window_started_at=excluded.window_started_at,computed_at=excluded.computed_at`, cutoff, cutoff, cutoff, p.cfg.TrendCookWeight, p.cfg.TrendFavoriteWeight, p.cfg.TrendReviewWeight, cutoff, now).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM recipe_trend_scores WHERE computed_at<? OR score=0", now).Error; err != nil {
			return err
		}
		return tx.Exec("DELETE FROM trend_projection_queue").Error
	})
}

type ReminderWorker struct {
	db    *gorm.DB
	hour  int
	clock platform.Clock
}

func NewReminderWorker(db *gorm.DB, hour int) *ReminderWorker {
	return &ReminderWorker{db: db, hour: hour, clock: platform.RealClock{}}
}
func (w *ReminderWorker) RunOnce(ctx context.Context) error {
	now := w.clock.Now().UTC()
	var users []struct {
		ID        uuid.UUID
		LocalDate time.Time
	}
	if err := w.db.WithContext(ctx).Raw(`SELECT id,((? AT TIME ZONE timezone)::date) local_date FROM users WHERE deactivated_at IS NULL AND current_streak>0 AND streak_last_qualifying_date=((? AT TIME ZONE timezone)::date-1) AND extract(hour from (? AT TIME ZONE timezone))>=? AND NOT EXISTS(SELECT 1 FROM cook_sessions cs WHERE cs.user_id=users.id AND cs.status='completed' AND cs.completion_local_date=((? AT TIME ZONE users.timezone)::date))`, now, now, now, w.hour, now).Scan(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return notify.PersistOptional(ctx, tx, u.ID, "streak", "streak_at_risk", fmt.Sprintf("streak-risk:%s:%s", u.ID, u.LocalDate.Format("2006-01-02")), map[string]any{"local_date": u.LocalDate.Format("2006-01-02")})
		})
		if err != nil {
			return err
		}
	}
	return nil
}
