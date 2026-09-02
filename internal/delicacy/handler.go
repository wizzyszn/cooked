package delicacy

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

type Handler struct {
	service *Service
}

func NewDelicacyHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) CreateDelicacy(ctx *gin.Context) {
	var req CreateDelicacyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.WriteAppError(ctx, apperrors.ErrValidation)
		return
	}
	userID, exists := ctx.Get(middlewares.ContextUserID)
	if !exists {
		models.WriteAppError(ctx, apperrors.ErrForbidden)
		return
	}
	if userID.(string) == "" {
		models.WriteAppError(ctx, apperrors.ErrForbidden)
		return
	}
	parsedUserID, err := uuid.Parse(userID.(string))
	if err != nil {
		models.WriteAppError(ctx, apperrors.ErrInternalServerError)
		return
	}

	delicacy, err := h.service.CreateDelicacy(ctx, &req, &parsedUserID)
	if err != nil {
		models.WriteAppError(ctx, err)
		return
	}
	models.WriteOk(ctx, delicacy)
}

func (h *Handler) GetDelicacy()    {}
func (h *Handler) UpdateDelicacy() {}
func (h *Handler) DeleteDelicacy() {}
