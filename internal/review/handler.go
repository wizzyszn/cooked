package review

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s: s} }
func id(c *gin.Context, name string) (uuid.UUID, error) {
	v, e := uuid.Parse(c.Param(name))
	if e != nil {
		return uuid.Nil, apperrors.ErrValidation
	}
	return v, nil
}
func current(c *gin.Context) (*domain.User, error) {
	u, ok := middlewares.CurrentUserFromContext(c)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}
	return u, nil
}
func view(c *gin.Context) (*uuid.UUID, bool) {
	u, ok := middlewares.CurrentUserFromContext(c)
	if !ok {
		return nil, false
	}
	return &u.ID, u.HasRole(domain.RoleModerator) || u.HasRole(domain.RoleAdmin)
}

func (h *Handler) Create(c *gin.Context) {
	version, e := id(c, "id")
	var q WriteRequest
	if e == nil && c.ShouldBindJSON(&q) != nil {
		e = apperrors.ErrValidation
	}
	u, _ := current(c)
	if e == nil {
		var out *domain.Review
		out, e = h.s.Create(c, u.ID, version, c.GetHeader("Idempotency-Key"), q)
		if e == nil {
			models.WriteCreated(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *Handler) Edit(c *gin.Context) {
	reviewID, e := id(c, "id")
	var q WriteRequest
	if e == nil && c.ShouldBindJSON(&q) != nil {
		e = apperrors.ErrValidation
	}
	u, _ := current(c)
	if e == nil {
		var out *domain.Review
		out, e = h.s.Edit(c, u.ID, reviewID, q)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *Handler) Get(c *gin.Context) {
	reviewID, e := id(c, "id")
	if e == nil {
		u, staff := view(c)
		var out *domain.Review
		out, e = h.s.Get(c, reviewID, u, staff)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *Handler) List(c *gin.Context) {
	version, e := id(c, "id")
	if e == nil {
		u, staff := view(c)
		var out *ReviewList
		out, e = h.s.List(c, version, u, staff)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *Handler) Report(c *gin.Context) {
	var q ReportRequest
	e := c.ShouldBindJSON(&q)
	u, _ := current(c)
	if e == nil {
		var out *Report
		out, e = h.s.Report(c, u.ID, c.GetHeader("Idempotency-Key"), q)
		if e == nil {
			models.WriteCreated(c, out)
			return
		}
	} else {
		e = apperrors.ErrValidation
	}
	models.WriteAppError(c, e)
}
func (h *Handler) Queue(c *gin.Context) {
	out, e := h.s.Queue(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Moderate(c *gin.Context) {
	reportID, e := id(c, "id")
	var q ModerationRequest
	if e == nil && c.ShouldBindJSON(&q) != nil {
		e = apperrors.ErrValidation
	}
	u, _ := current(c)
	if e == nil {
		e = h.s.Moderate(c, u.ID, reportID, q)
	}
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
