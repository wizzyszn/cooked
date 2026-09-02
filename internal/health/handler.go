package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wizzyszn/cooked/pkg/models"
	"gorm.io/gorm"
)

const readinessTimeout = 2 * time.Second

type Handler struct {
	db              *gorm.DB
	requiredVersion uint
}

func NewHandler(db *gorm.DB, requiredVersion uint) *Handler {
	return &Handler{db: db, requiredVersion: requiredVersion}
}

func (h *Handler) Live(ctx *gin.Context) {
	models.WriteOk(ctx, gin.H{"status": "live"})
}

func (h *Handler) Ready(ctx *gin.Context) {
	if h.db == nil {
		h.notReady(ctx, "Database is unavailable")
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx.Request.Context(), readinessTimeout)
	defer cancel()
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(checkCtx) != nil {
		h.notReady(ctx, "Database is unavailable")
		return
	}

	var state struct {
		Version uint
		Dirty   bool
	}
	err = h.db.WithContext(checkCtx).Raw("SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&state).Error
	if err != nil || state.Dirty || state.Version < h.requiredVersion {
		h.notReady(ctx, "Database migrations are not current")
		return
	}

	models.WriteOk(ctx, gin.H{"status": "ready", "migration_version": state.Version})
}

func (h *Handler) notReady(ctx *gin.Context, message string) {
	ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, models.ErrorResponse(ctx, "NOT_READY", message, http.StatusServiceUnavailable))
}
