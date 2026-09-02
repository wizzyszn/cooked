package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/auth"
	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/testutil"
)

type fakeCurrentUserLoader struct {
	user *domain.User
	err  error
}

func (f fakeCurrentUserLoader) FindByID(context.Context, uuid.UUID) (*domain.User, error) {
	return f.user, f.err
}

func authTestManager() *auth.JWTManager {
	return auth.NewJWTManager(config.JWTConfig{
		AccessSecret:        "middleware-access-secret",
		RefreshSecret:       "middleware-refresh-secret",
		AccessTTLMin:        15,
		RefreshTLLDay:       14,
		EmailVerifyTTLHours: 24,
	})
}

func performAuthorizedRequest(t *testing.T, user *domain.User, handlers ...gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	manager := authTestManager()
	token, _, err := manager.IssueAccessToken(user.ID, user.Email, user.IsVerified)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	router := gin.New()
	router.GET("/test", append([]gin.HandlerFunc{RequireAuth(manager, fakeCurrentUserLoader{user: user})}, handlers...)...)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestUnverifiedUserCanAuthenticateButCannotPassVerifiedGate(t *testing.T) {
	user := testutil.NewUser().Unverified().Build()

	authenticated := performAuthorizedRequest(t, user, func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})
	if authenticated.Code != http.StatusNoContent {
		t.Fatalf("authenticated route status = %d, want %d", authenticated.Code, http.StatusNoContent)
	}

	verified := performAuthorizedRequest(t, user, RequireVerified(), func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})
	if verified.Code != http.StatusForbidden {
		t.Fatalf("verified route status = %d, want %d", verified.Code, http.StatusForbidden)
	}
}

func TestRequireRoleAuthorizationMatrix(t *testing.T) {
	tests := []struct {
		name       string
		user       *domain.User
		required   domain.Role
		wantStatus int
	}{
		{name: "user allowed as user", user: testutil.NewUser().Build(), required: domain.RoleUser, wantStatus: http.StatusNoContent},
		{name: "user denied moderator", user: testutil.NewUser().Build(), required: domain.RoleModerator, wantStatus: http.StatusForbidden},
		{name: "moderator allowed", user: testutil.NewUser().WithRole(domain.RoleModerator).Build(), required: domain.RoleModerator, wantStatus: http.StatusNoContent},
		{name: "moderator denied admin", user: testutil.NewUser().WithRole(domain.RoleModerator).Build(), required: domain.RoleAdmin, wantStatus: http.StatusForbidden},
		{name: "admin allowed admin", user: testutil.NewUser().WithRole(domain.RoleAdmin).Build(), required: domain.RoleAdmin, wantStatus: http.StatusNoContent},
		{name: "admin allowed moderator", user: testutil.NewUser().WithRole(domain.RoleAdmin).Build(), required: domain.RoleModerator, wantStatus: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performAuthorizedRequest(t, tt.user, RequireRole(tt.required), func(ctx *gin.Context) {
				ctx.Status(http.StatusNoContent)
			})
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}
