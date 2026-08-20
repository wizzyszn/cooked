package middlewares

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wizzyszn/cooked/internal/auth"
	"github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

var (
	ContextUserID    = "userID"
	ContextUserEmail = "userEmail"
	ContextClaims    = "claims"
)

func RequireAuth(jwt *auth.JWTManager) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		rawAuthHeader := ctx.GetHeader("Authorization")
		if rawAuthHeader == "" {
			models.WriteAppError(ctx, errors.ErrUnAuthorized)
			return
		}
		parts := strings.Fields(rawAuthHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			models.WriteAppError(ctx, errors.ErrUnAuthorized)
			return
		}
		token := parts[1]
		if token == "" {
			models.WriteAppError(ctx, errors.ErrInvalidToken)
			return
		}
		claims, err := jwt.Parse(token)
		if err != nil {
			models.WriteAppError(ctx, err)
			return
		}
		if !claims.EmailVerified {
			models.WriteAppError(ctx, errors.ErrUnAuthorized)
			return
		}
		ctx.Set(ContextUserEmail, claims.Email)
		ctx.Set(ContextUserID, claims.UserID)
		ctx.Set(ContextClaims, claims)
		ctx.Next()
	}
}
