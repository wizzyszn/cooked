package delicacy

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
	"net/http"
)

type Handler struct{ service *Service }

func NewDelicacyHandler(s *Service) *Handler { return &Handler{service: s} }
func actor(c *gin.Context) (uuid.UUID, error) {
	u, ok := middlewares.CurrentUserFromContext(c)
	if !ok {
		return uuid.Nil, apperrors.ErrUnauthorized
	}
	return u.ID, nil
}
func idParam(c *gin.Context) (uuid.UUID, error) {
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return uuid.Nil, apperrors.ErrValidation
	}
	return id, nil
}
func (h *Handler) List(c *gin.Context) {
	v, e := h.service.List(c, c.Query("category"), c.Query("region"))
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) Pending(c *gin.Context) {
	v, e := h.service.Pending(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) Get(c *gin.Context) {
	id, e := idParam(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	v, e := h.service.Get(c, id)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) Similar(c *gin.Context) {
	v, e := h.service.Similar(c, c.Query("name"))
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) Create(publish bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req WriteRequest
		if e := c.ShouldBindJSON(&req); e != nil {
			models.WriteAppError(c, apperrors.ErrValidation)
			return
		}
		a, e := actor(c)
		if e != nil {
			models.WriteAppError(c, e)
			return
		}
		v, e := h.service.Create(c, req, a, publish)
		if e != nil {
			models.WriteAppError(c, e)
			return
		}
		models.WriteCreated(c, v)
	}
}
func (h *Handler) Edit(c *gin.Context) {
	id, e := idParam(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	var req WriteRequest
	if e = c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	a, _ := actor(c)
	v, e := h.service.EditPending(c, id, a, req)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) Withdraw(c *gin.Context) {
	id, e := idParam(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	a, _ := actor(c)
	if e = h.service.Withdraw(c, id, a); e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) Moderate(status domain.DelicacyStatus) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, e := idParam(c)
		if e != nil {
			models.WriteAppError(c, e)
			return
		}
		var req ModerateRequest
		if e = c.ShouldBindJSON(&req); e != nil {
			models.WriteAppError(c, apperrors.ErrValidation)
			return
		}
		a, _ := actor(c)
		if e = h.service.Moderate(c, id, a, status, req.Reason); e != nil {
			models.WriteAppError(c, e)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
func (h *Handler) Merge(c *gin.Context) {
	id, e := idParam(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	var req MergeRequest
	if e = c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	a, _ := actor(c)
	if e = h.service.Merge(c, id, req.TargetID, a, req.Reason); e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) Taxonomies(c *gin.Context) {
	v, e := h.service.Taxonomies(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) WriteTaxonomy(c *gin.Context) {
	var req TaxonomyRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	var id *uuid.UUID
	if raw := c.Param("id"); raw != "" {
		v, e := uuid.Parse(raw)
		if e != nil {
			models.WriteAppError(c, apperrors.ErrValidation)
			return
		}
		id = &v
	}
	a, _ := actor(c)
	v, e := h.service.WriteTaxonomy(c, c.Param("kind"), id, req, a)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) RetireTaxonomy(c *gin.Context) {
	id, e := idParam(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	var req ModerateRequest
	if e = c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	a, _ := actor(c)
	if e = h.service.RetireTaxonomy(c, c.Param("kind"), id, a, req.Reason); e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}
