package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func (h *AuthHandler) Logout(ctx *gin.Context) {
	var req LogoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.WriteAppError(ctx, errors.ErrValidation)
		return
	}
	if err := h.service.Logout(ctx.Request.Context(), &req); err != nil {
		models.WriteAppError(ctx, err)
		return
	}
	models.WriteOk(ctx, gin.H{
		"message": "Logout successful.",
	})
}

func (h *AuthHandler) LogoutAll(ctx *gin.Context) {
	var req LogoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.WriteAppError(ctx, errors.ErrValidation)
		return
	}

	// Set by RequireAuth middleware (ContextUserID = "userID").
	userIDStr := ctx.GetString("userID")
	if userIDStr == "" {
		models.WriteAppError(ctx, errors.ErrUnauthorized)
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		models.WriteAppError(ctx, errors.ErrUnauthorized)
		return
	}

	if err := h.service.LogoutAll(ctx.Request.Context(), userID, &req); err != nil {
		models.WriteAppError(ctx, err)
		return
	}
	models.WriteOk(ctx, gin.H{
		"message": "Successfully logged out all related sessions.",
	})
}

func (h *AuthHandler) ForgotPassword(ctx *gin.Context) {
	var req ForgotPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.WriteAppError(ctx, errors.ErrValidation)
		return
	}

	if err := h.service.ForgotPassword(ctx.Request.Context(), &req); err != nil {
		models.WriteAppError(ctx, err)
		return
	}
	models.WriteOk(ctx, gin.H{
		"message": "If this email is valid, a reset otp has been sent to this email.",
	})
}

func (h *AuthHandler) ResetPassword(ctx *gin.Context) {
	var req ResetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.WriteAppError(ctx, errors.ErrValidation)
		return
	}
	if err := h.service.ResetPassword(ctx.Request.Context(), &req); err != nil {
		models.WriteAppError(ctx, err)
		return
	}
	models.WriteOk(ctx, gin.H{
		"message": "password reset successful",
	})
}
