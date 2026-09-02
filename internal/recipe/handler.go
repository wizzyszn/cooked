package recipe

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
	"strconv"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s: s} }
func viewer(c *gin.Context) (*uuid.UUID, bool) {
	u, ok := middlewares.CurrentUserFromContext(c)
	if !ok {
		return nil, false
	}
	return &u.ID, u.HasRole(domain.RoleModerator) || u.HasRole(domain.RoleAdmin)
}
func rid(c *gin.Context) (uuid.UUID, error) {
	v, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return uuid.Nil, apperrors.ErrValidation
	}
	return v, nil
}
func servings(c *gin.Context) *int {
	if v, e := strconv.Atoi(c.Query("servings")); e == nil && v > 0 {
		return &v
	}
	return nil
}
func (h *Handler) Get(c *gin.Context) {
	id, e := rid(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	u, staff := viewer(c)
	v, e := h.s.Get(c, id, u, staff, servings(c))
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) GetVersion(c *gin.Context) {
	id, e := rid(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	u, staff := viewer(c)
	v, e := h.s.GetVersion(c, id, u, staff, servings(c))
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) Create(c *gin.Context) {
	var q CreateRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	u, _ := viewer(c)
	v, e := h.s.Create(c, *u, q)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteCreated(c, v)
}
func (h *Handler) Draft(c *gin.Context) {
	id, e := rid(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	u, staff := viewer(c)
	v, e := h.s.Draft(c, id, *u, staff)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) Update(c *gin.Context) {
	id, e := rid(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	var q Snapshot
	if e = c.ShouldBindJSON(&q); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	u, staff := viewer(c)
	v, e := h.s.UpdateDraft(c, id, *u, q, staff)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) Publish(c *gin.Context) {
	id, e := rid(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	u, _ := viewer(c)
	v, e := h.s.Publish(c, id, *u, c.GetHeader("Idempotency-Key"))
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, v)
}
func (h *Handler) Visibility(c *gin.Context) {
	id, e := rid(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	var q VisibilityRequest
	if e = c.ShouldBindJSON(&q); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	u, _ := viewer(c)
	if e = h.s.Visibility(c, id, *u, q.Visibility); e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
func (h *Handler) Delete(c *gin.Context) {
	id, e := rid(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	u, _ := viewer(c)
	if e = h.s.Delete(c, id, *u); e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
