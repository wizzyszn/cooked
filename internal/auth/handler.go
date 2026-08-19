package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

type AuthHandler struct {
	service *AuthService
}

func NewAuthHandler(service *AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Register(ctx *gin.Context) {
	var req RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.WriteAppError(ctx, errors.ErrValidation)
		return
	}

	res, err := h.service.Register(ctx.Request.Context(), &req)
	if err != nil {
		models.WriteAppError(ctx, err)
		return
	}
	models.WriteCreated(ctx, res)
}

func (h *AuthHandler) VerifyEmail(ctx *gin.Context) {
	token := ctx.Query("token")
	if token == "" {
		models.WriteAppError(ctx, errors.ErrValidation)
		return
	}

	res, err := h.service.VerifyEmail(ctx.Request.Context(), token)
	if err != nil {
		models.WriteAppError(ctx, err)
		return
	}
	models.WriteOk(ctx, res)
}

func (h *AuthHandler) Login(ctx *gin.Context) {
	var req LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.WriteAppError(ctx, errors.ErrValidation)
		return
	}
	res, err := h.service.Login(ctx.Request.Context(), &req)
	if err != nil {
		models.WriteAppError(ctx, err)
		return
	}
	models.WriteOk(ctx, res)
}

func (h *AuthHandler) Refresh(ctx *gin.Context) {
	var req RefreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.WriteAppError(ctx, errors.ErrValidation)
		return
	}
	res, err := h.service.Refresh(ctx.Request.Context(), &req)
	if err != nil {
		models.WriteAppError(ctx, err)
		return
	}
	models.WriteOk(ctx, res)
}
