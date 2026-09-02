package router

import (
	"github.com/gin-gonic/gin"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"github.com/wizzyszn/cooked/internal/app"
	"github.com/wizzyszn/cooked/internal/auth"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/delicacy"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/health"
	"github.com/wizzyszn/cooked/internal/media"
	"github.com/wizzyszn/cooked/internal/user"
)

func Init(deps *app.Dependencies) *gin.Engine {
	r := gin.New()
	var proxies []string
	if deps != nil && deps.Config != nil {
		proxies = deps.Config.Server.TrustedProxies
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		if deps != nil && deps.Logger != nil {
			deps.Logger.Warnw("invalid TRUSTED_PROXIES; trusting no proxies", "error", err)
		}
		_ = r.SetTrustedProxies(nil)
	}
	r.Use(gin.Recovery())
	r.Use(middlewares.RequestId())
	r.Use(middlewares.RequestLogger(deps.Logger))
	r.Use(middlewares.NewRateLimiter(100).Limit)

	healthHandler := health.NewHandler(deps.Database, db.LatestMigrationVersion)
	r.GET("/health/live", healthHandler.Live)
	r.GET("/health/ready", healthHandler.Ready)

	v1 := r.Group("/api/v1")
	authG := v1.Group("/auth")
	authHandler := auth.NewAuthHandler(deps.AuthService)
	googleHandler := auth.NewGoogleHandler(deps.GoogleService)

	reqAuthM := middlewares.RequireAuth(deps.Tokens, deps.Users)
	//onboarding
	{
		authG.POST("/register", authHandler.Register)
		authG.GET("/verify-email", authHandler.VerifyEmail)
		authG.POST("/login", middlewares.NewRateLimiter(5).Limit, authHandler.Login)
		authG.POST("/refresh", authHandler.Refresh)
		authG.POST("/logout", authHandler.Logout)
		authG.POST("/forgot-password", middlewares.NewRateLimiter(5).Limit, authHandler.ForgotPassword)
		authG.POST("/reset-password", middlewares.NewRateLimiter(5).Limit, authHandler.ResetPassword)
		authG.POST("/google/start", googleHandler.Start)
		authG.GET("/google/callback", googleHandler.Callback)
		authG.POST("/google/exchange", googleHandler.Exchange)
	}
	authed := v1.Use(reqAuthM)
	{
		authed.POST("/auth/logout-all", authHandler.LogoutAll)
		userHandler := user.NewHandler(deps.UserService)
		authed.GET("/users/me", userHandler.Me)
		authed.PATCH("/users/me", userHandler.Update)
		authed.PUT("/users/me/dietary-preferences", userHandler.Dietary)
		authed.DELETE("/users/me", userHandler.Delete)
		admin := v1.Group("/admin", reqAuthM, middlewares.RequireRole(domain.RoleAdmin))
		admin.PUT("/users/:id/roles/:role", userHandler.GrantRole)
		admin.DELETE("/users/:id/roles/:role", userHandler.RevokeRole)
	}
	if deps.MediaService != nil {
		mediaHandler := media.NewHandler(deps.MediaService)
		v1.GET("/media/:id", mediaHandler.PublicGet)
		authed.POST("/media/uploads", mediaHandler.Initialize)
		authed.POST("/media/:id/complete", mediaHandler.Complete)
		authed.GET("/media/:id/access", mediaHandler.OwnerGet)
	}
	userHandler := user.NewHandler(deps.UserService)
	v1.GET("/profiles/:username", userHandler.Profile)
	//Delicacy
	delicacyG := v1.Group("/delicacy")
	delicacyHandler := delicacy.NewDelicacyHandler(deps.DelicacyService)
	authed = delicacyG.Use(reqAuthM, middlewares.RequireVerified())
	{
		authed.POST("", delicacyHandler.CreateDelicacy)
	}
	return r
}
