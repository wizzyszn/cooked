package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func currentID(c *gin.Context) (uuid.UUID, error) {
	value, ok := c.Get("currentUser")
	u, valid := value.(*domain.User)
	if !ok || !valid || u == nil {
		return uuid.Nil, apperrors.ErrUnauthorized
	}
	return u.ID, nil
}

func (h *Handler) Me(c *gin.Context) {
	id, e := currentID(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	out, e := h.service.Me(c.Request.Context(), id)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Update(c *gin.Context) {
	id, e := currentID(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	var req UpdateProfileRequest
	if e = c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	out, e := h.service.UpdateProfile(c.Request.Context(), id, req)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Dietary(c *gin.Context) {
	id, e := currentID(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	var req DietaryPreferencesRequest
	if e = c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	out, e := h.service.ReplaceDietary(c.Request.Context(), id, req)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Profile(c *gin.Context) {
	out, e := h.service.PublicProfile(c.Request.Context(), c.Param("username"))
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *Handler) Delete(c *gin.Context) {
	id, e := currentID(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	if e = h.service.Anonymize(c.Request.Context(), id); e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) GrantRole(c *gin.Context)  { h.role(c, true) }
func (h *Handler) RevokeRole(c *gin.Context) { h.role(c, false) }
func (h *Handler) role(c *gin.Context, grant bool) {
	actor, e := currentID(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	target, e := uuid.Parse(c.Param("id"))
	if e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	role := domain.Role(c.Param("role"))
	var req RoleChangeRequest
	if e = c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	if e = h.service.SetRole(c.Request.Context(), actor, target, role, grant, req.Reason); e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, gin.H{"role": role, "granted": grant})
}
