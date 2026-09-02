package media

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func mediaUser(c *gin.Context) (uuid.UUID, error) {
	v, ok := c.Get("currentUser")
	u, valid := v.(*domain.User)
	if !ok || !valid || u == nil {
		return uuid.Nil, apperrors.ErrUnauthorized
	}
	return u.ID, nil
}
func (h *Handler) Initialize(c *gin.Context) {
	id, e := mediaUser(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	var req InitializeRequest
	if e = c.ShouldBindJSON(&req); e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	out, e := h.service.Initialize(c.Request.Context(), id, req)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteCreated(c, out)
}
func (h *Handler) Complete(c *gin.Context) {
	owner, e := mediaUser(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	out, e := h.service.CompleteUpload(c.Request.Context(), owner, id)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.JSON(http.StatusAccepted, models.SuccessResponse(c, out))
}
func (h *Handler) PublicGet(c *gin.Context) {
	var id *uuid.UUID
	if u, ok := middlewares.CurrentUserFromContext(c); ok {
		id = &u.ID
	}
	h.get(c, id)
}
func (h *Handler) OwnerGet(c *gin.Context) {
	id, e := mediaUser(c)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	h.get(c, &id)
}
func (h *Handler) get(c *gin.Context, userID *uuid.UUID) {
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	out, e := h.service.Get(c.Request.Context(), userID, id)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
