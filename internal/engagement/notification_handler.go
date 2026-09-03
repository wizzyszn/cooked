package engagement

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"github.com/wizzyszn/cooked/internal/notify"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

type NotificationHandler struct{ service *notify.Service }

func NewNotificationHandler(service *notify.Service) *NotificationHandler {
	return &NotificationHandler{service: service}
}
func notificationUser(c *gin.Context) (uuid.UUID, error) {
	u, ok := middlewares.CurrentUserFromContext(c)
	if !ok {
		return uuid.Nil, apperrors.ErrUnauthorized
	}
	return u.ID, nil
}
func (h *NotificationHandler) Preferences(c *gin.Context) {
	u, e := notificationUser(c)
	if e == nil {
		var out []notify.Preference
		out, e = h.service.Preferences(c, u)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *NotificationHandler) SetPreference(c *gin.Context) {
	u, e := notificationUser(c)
	var q notify.PreferenceRequest
	if e == nil && c.ShouldBindJSON(&q) != nil {
		e = apperrors.ErrValidation
	}
	if e == nil {
		e = h.service.SetPreference(c, u, q)
	}
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
func (h *NotificationHandler) Inbox(c *gin.Context) {
	u, e := notificationUser(c)
	if e == nil {
		var out notify.Inbox
		out, e = h.service.Inbox(c, u)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	u, e := notificationUser(c)
	var id uuid.UUID
	if e == nil {
		id, e = uuid.Parse(c.Param("id"))
		if e != nil {
			e = apperrors.ErrValidation
		}
	}
	if e == nil {
		e = h.service.MarkRead(c, u, id)
	}
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
