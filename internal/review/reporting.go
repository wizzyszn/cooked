package review

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/platform"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"gorm.io/gorm"
)

func (s *Service) Report(ctx context.Context, actor uuid.UUID, key string, q ReportRequest) (*Report, error) {
	if _, err := platform.ParseIdempotencyKey(key); err != nil || (q.TargetType != "recipe" && q.TargetType != "review") || q.TargetID == uuid.Nil || !reportReasons[q.Reason] || len(strings.TrimSpace(q.Details)) > 2000 {
		return nil, apperrors.ErrValidation
	}
	var out Report
	err := db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		var verified bool
		if err := tx.Raw("SELECT is_verified FROM users WHERE id=? AND deactivated_at IS NULL", actor).Scan(&verified).Error; err != nil {
			return err
		}
		if !verified {
			return apperrors.ErrEmailNotVerified
		}
		var priorRaw string
		if err := tx.Raw("SELECT report_id::text FROM content_report_commands WHERE reporter_id=? AND idempotency_key=?", actor, key).Scan(&priorRaw).Error; err != nil {
			return err
		}
		if priorRaw != "" {
			prior, err := uuid.Parse(priorRaw)
			if err != nil {
				return err
			}
			return tx.Table("content_reports").First(&out, "id=?", prior).Error
		}
		recipeID, _, status, versionID, err := lockTarget(tx, q.TargetType, q.TargetID)
		if err != nil || status != "visible" {
			return apperrors.ErrNotFound
		}
		if ok, _ := accessible(tx, recipeID, &actor, false); !ok {
			return apperrors.ErrNotFound
		}
		out = Report{ID: uuid.New(), ReporterID: actor, TargetType: q.TargetType, TargetID: q.TargetID, Reason: q.Reason, Details: strings.TrimSpace(q.Details), State: "pending", CreatedAt: time.Now().UTC()}
		if err := tx.Table("content_reports").Create(&out).Error; err != nil {
			return apperrors.ErrConflict
		}
		if err := tx.Exec("INSERT INTO content_report_commands(reporter_id,idempotency_key,report_id) VALUES (?,?,?)", actor, key, out.ID).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Raw("SELECT count(*) FROM content_reports cr JOIN users u ON u.id=cr.reporter_id WHERE cr.target_type=? AND cr.target_id=? AND cr.state='pending' AND u.is_verified=true", q.TargetType, q.TargetID).Scan(&count).Error; err != nil {
			return err
		}
		if count == 3 {
			if q.TargetType == "recipe" {
				err = tx.Model(&domain.Recipe{}).Where("id=? AND moderation_status='visible'", q.TargetID).Update("moderation_status", domain.RecipeHidden).Error
			} else {
				err = tx.Model(&domain.Review{}).Where("id=? AND moderation_status='visible'", q.TargetID).Update("moderation_status", domain.ReviewHidden).Error
			}
			if err != nil {
				return err
			}
			if versionID != uuid.Nil {
				if err := recompute(tx, versionID); err != nil {
					return err
				}
			}
			before, _ := json.Marshal(map[string]any{"moderation_status": "visible"})
			after, _ := json.Marshal(map[string]any{"moderation_status": "hidden", "eligible_reports": 3})
			if err := tx.Create(&domain.AuditLog{Action: "auto_hide", TargetType: q.TargetType, TargetID: &q.TargetID, Reason: "three distinct verified reports", BeforeJSON: before, AfterJSON: after}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return &out, err
}

func lockTarget(tx *gorm.DB, kind string, id uuid.UUID) (recipeID, authorID uuid.UUID, status string, versionID uuid.UUID, err error) {
	if kind == "recipe" {
		var row struct {
			RecipeID, AuthorID uuid.UUID
			Status             string
		}
		err = tx.Raw("SELECT id recipe_id,user_id author_id,moderation_status status FROM recipes WHERE id=? AND deleted_at IS NULL FOR UPDATE", id).Scan(&row).Error
		return row.RecipeID, row.AuthorID, row.Status, uuid.Nil, err
	}
	var row struct {
		RecipeID, AuthorID, VersionID uuid.UUID
		Status                        string
	}
	err = tx.Raw("SELECT recipe_id,user_id author_id,recipe_version_id version_id,moderation_status status FROM reviews WHERE id=? FOR UPDATE", id).Scan(&row).Error
	return row.RecipeID, row.AuthorID, row.Status, row.VersionID, err
}

func (s *Service) Queue(ctx context.Context) ([]Report, error) {
	out := []Report{}
	err := s.repo.DB().WithContext(ctx).Table("content_reports").Where("state='pending'").Order("created_at,id").Find(&out).Error
	return out, err
}

func (s *Service) Moderate(ctx context.Context, actor, reportID uuid.UUID, q ModerationRequest) error {
	q.Reason = strings.TrimSpace(q.Reason)
	if q.Reason == "" || (q.Action != "restore" && q.Action != "keep_hidden" && q.Action != "remove") {
		return apperrors.ErrValidation
	}
	return db.WithinTransaction(ctx, s.repo.DB(), func(tx *gorm.DB) error {
		var report Report
		if err := tx.Raw("SELECT * FROM content_reports WHERE id=? AND state='pending' FOR UPDATE", reportID).Scan(&report).Error; err != nil || report.ID == uuid.Nil {
			return apperrors.ErrNotFound
		}
		recipeID, authorID, before, versionID, err := lockTarget(tx, report.TargetType, report.TargetID)
		_ = recipeID
		if err != nil || authorID == uuid.Nil {
			return apperrors.ErrNotFound
		}
		after := before
		state := "upheld"
		switch q.Action {
		case "restore":
			after = "visible"
			state = "dismissed"
		case "keep_hidden":
			after = "hidden"
		case "remove":
			after = "removed"
		}
		if report.TargetType == "recipe" {
			if err = tx.Model(&domain.Recipe{}).Where("id=?", report.TargetID).Update("moderation_status", after).Error; err != nil {
				return err
			}
		} else {
			if err = tx.Model(&domain.Review{}).Where("id=?", report.TargetID).Update("moderation_status", after).Error; err != nil {
				return err
			}
			if err = recompute(tx, versionID); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if err = tx.Table("content_reports").Where("target_type=? AND target_id=? AND state='pending'", report.TargetType, report.TargetID).Updates(map[string]any{"state": state, "resolved_at": now}).Error; err != nil {
			return err
		}
		beforeJSON, _ := json.Marshal(map[string]any{"moderation_status": before})
		afterJSON, _ := json.Marshal(map[string]any{"moderation_status": after})
		if err = tx.Create(&domain.AuditLog{ActorID: &actor, Action: q.Action, TargetType: report.TargetType, TargetID: &report.TargetID, Reason: q.Reason, BeforeJSON: beforeJSON, AfterJSON: afterJSON}).Error; err != nil {
			return err
		}
		return inApp(tx, authorID, "moderation_outcome", map[string]any{"target_type": report.TargetType, "target_id": report.TargetID, "action": q.Action, "reason": q.Reason})
	})
}
