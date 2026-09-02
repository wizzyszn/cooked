package middlewares

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/auth"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

var (
	ContextUserID    = "userID"
	ContextUserEmail = "userEmail"
	ContextClaims    = "claims"
	ContextUser      = "currentUser"
)

type CurrentUserLoader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

func RequireAuth(jwt *auth.JWTManager, users CurrentUserLoader) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if jwt == nil || users == nil {
			models.WriteAppError(ctx, errors.ErrServiceUnavailable)
			return
		}
		rawAuthHeader := ctx.GetHeader("Authorization")
		if rawAuthHeader == "" {
			models.WriteAppError(ctx, errors.ErrUnauthorized)
			return
		}
		parts := strings.Fields(rawAuthHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			models.WriteAppError(ctx, errors.ErrUnauthorized)
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
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			models.WriteAppError(ctx, errors.ErrInvalidToken)
			return
		}
		currentUser, err := users.FindByID(ctx.Request.Context(), userID)
		if err != nil {
			models.WriteAppError(ctx, errors.ErrInternalServerError)
			return
		}
		if currentUser == nil || currentUser.DeactivatedAt != nil {
			models.WriteAppError(ctx, errors.ErrUnauthorized)
			return
		}

		ctx.Set(ContextUserEmail, currentUser.Email)
		ctx.Set(ContextUserID, currentUser.ID.String())
		ctx.Set(ContextClaims, claims)
		ctx.Set(ContextUser, currentUser)
		ctx.Next()
	}
}

func RequireVerified() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentUser, ok := CurrentUserFromContext(ctx)
		if !ok {
			models.WriteAppError(ctx, errors.ErrUnauthorized)
			return
		}
		if !currentUser.IsVerified {
			models.WriteAppError(ctx, errors.ErrEmailNotVerified)
			return
		}
		ctx.Next()
	}
}

func RequireRole(roles ...domain.Role) gin.HandlerFunc {
	allowed := make(map[domain.Role]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(ctx *gin.Context) {
		currentUser, ok := CurrentUserFromContext(ctx)
		if !ok {
			models.WriteAppError(ctx, errors.ErrUnauthorized)
			return
		}
		if currentUser.HasRole(domain.RoleAdmin) {
			ctx.Next()
			return
		}
		for role := range allowed {
			if currentUser.HasRole(role) {
				ctx.Next()
				return
			}
		}
		models.WriteAppError(ctx, errors.ErrForbidden)
	}
}

func CurrentUserFromContext(ctx *gin.Context) (*domain.User, bool) {
	value, exists := ctx.Get(ContextUser)
	if !exists {
		return nil, false
	}
	currentUser, ok := value.(*domain.User)
	return currentUser, ok && currentUser != nil
}
