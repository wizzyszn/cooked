package cook

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
	"strconv"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s: s} }
func actor(c *gin.Context) (uuid.UUID, error) {
	u, ok := middlewares.CurrentUserFromContext(c)
	if !ok {
		return uuid.Nil, apperrors.ErrUnauthorized
	}
	return u.ID, nil
}
func pathID(c *gin.Context, name string) (uuid.UUID, error) {
	id, e := uuid.Parse(c.Param(name))
	if e != nil {
		return uuid.Nil, apperrors.ErrValidation
	}
	return id, nil
}
func (h *Handler) Start(c *gin.Context) {
	var req StartRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	u, _ := actor(c)
	out, e := h.s.Start(c.Request.Context(), u, req.RecipeVersionID)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Active(c *gin.Context) {
	v, e := uuid.Parse(c.Query("recipe_version_id"))
	if e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	u, _ := actor(c)
	out, e := h.s.Active(c.Request.Context(), u, v)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Get(c *gin.Context) {
	id, e := pathID(c, "id")
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	u, _ := actor(c)
	out, e := h.s.Session(c.Request.Context(), u, id)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Visit(c *gin.Context) {
	id, e := pathID(c, "id")
	step, se := pathID(c, "stepId")
	if e == nil {
		e = se
	}
	u, _ := actor(c)
	if e == nil {
		e = h.s.Visit(c.Request.Context(), u, id, step)
	}
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
func (h *Handler) Timer(c *gin.Context) {
	id, e := pathID(c, "id")
	step, se := pathID(c, "stepId")
	if e == nil {
		e = se
	}
	var req TimerRequest
	if e == nil {
		if bind := c.ShouldBindJSON(&req); bind != nil {
			e = apperrors.ErrValidation
		}
	}
	u, _ := actor(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	out, e := h.s.Timer(c.Request.Context(), u, id, step, req)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Abandon(c *gin.Context) {
	id, e := pathID(c, "id")
	u, _ := actor(c)
	if e == nil {
		e = h.s.Abandon(c.Request.Context(), u, id)
	}
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
func (h *Handler) Complete(c *gin.Context) {
	id, e := pathID(c, "id")
	var req CompleteRequest
	if e == nil && c.Request.ContentLength != 0 {
		if bind := c.ShouldBindJSON(&req); bind != nil {
			e = apperrors.ErrValidation
		}
	}
	u, _ := actor(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	out, e := h.s.Complete(c.Request.Context(), u, id, c.GetHeader("Idempotency-Key"), req.PhotoMediaID)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) List(c *gin.Context) {
	limit := 0
	var e error
	if raw := c.Query("limit"); raw != "" {
		limit, e = strconv.Atoi(raw)
	}
	u, _ := actor(c)
	if e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	out, e := h.s.List(c.Request.Context(), u, c.Query("cursor"), limit)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Ingest(c *gin.Context) {
	var req EventRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	var userID *uuid.UUID
	if u, ok := middlewares.CurrentUserFromContext(c); ok {
		userID = &u.ID
	}
	if e := h.s.Ingest(c.Request.Context(), userID, req); e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
func (h *Handler) Metrics(c *gin.Context) {
	out, e := h.s.Metrics(c.Request.Context())
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
