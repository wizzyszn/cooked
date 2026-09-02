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
	"github.com/wizzyszn/cooked/internal/recipe"
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
		v1.GET("/media/:id", middlewares.OptionalAuth(deps.Tokens, deps.Users), mediaHandler.PublicGet)
		authed.POST("/media/uploads", mediaHandler.Initialize)
		authed.POST("/media/:id/complete", mediaHandler.Complete)
		authed.GET("/media/:id/access", mediaHandler.OwnerGet)
	}
	userHandler := user.NewHandler(deps.UserService)
	v1.GET("/profiles/:username", userHandler.Profile)
	// Delicacy is the API's backward-compatible name for the curated Dish catalog.
	delicacyG := v1.Group("/delicacies")
	delicacyHandler := delicacy.NewDelicacyHandler(deps.DelicacyService)
	delicacyG.GET("", delicacyHandler.List)
	delicacyG.GET("/duplicate-suggestions", delicacyHandler.Similar)
	delicacyG.GET("/:id", delicacyHandler.Get)
	v1.GET("/taxonomies", delicacyHandler.Taxonomies)
	verified := delicacyG.Group("", reqAuthM, middlewares.RequireVerified())
	{
		verified.POST("", delicacyHandler.Create(false))
		verified.PATCH("/:id", delicacyHandler.Edit)
		verified.POST("/:id/withdraw", delicacyHandler.Withdraw)
	}
	staff := v1.Group("/staff", reqAuthM, middlewares.RequireRole(domain.RoleModerator, domain.RoleAdmin))
	staff.POST("/delicacies", delicacyHandler.Create(true))
	staff.GET("/delicacies", delicacyHandler.Pending)
	staff.POST("/delicacies/:id/approve", delicacyHandler.Moderate(domain.DelicacyPublished))
	staff.POST("/delicacies/:id/reject", delicacyHandler.Moderate(domain.DelicacyRejected))
	staff.GET("/taxonomies", delicacyHandler.Taxonomies)
	staff.POST("/taxonomies/:kind", delicacyHandler.WriteTaxonomy)
	staff.PUT("/taxonomies/:kind/:id", delicacyHandler.WriteTaxonomy)
	staff.POST("/taxonomies/:kind/:id/retire", delicacyHandler.RetireTaxonomy)
	adminDish := v1.Group("/admin/delicacies", reqAuthM, middlewares.RequireRole(domain.RoleAdmin))
	adminDish.POST("/:id/merge", delicacyHandler.Merge)
	recipeHandler := recipe.NewHandler(deps.RecipeService)
	v1.GET("/recipes/:id", middlewares.OptionalAuth(deps.Tokens, deps.Users), recipeHandler.Get)
	v1.GET("/recipe-versions/:id", middlewares.OptionalAuth(deps.Tokens, deps.Users), recipeHandler.GetVersion)
	recipeAuthor := v1.Group("/recipes", reqAuthM)
	recipeAuthor.POST("", recipeHandler.Create)
	recipeAuthor.GET("/:id/draft", recipeHandler.Draft)
	recipeAuthor.PUT("/:id/draft", recipeHandler.Update)
	recipeAuthor.POST("/:id/publish", middlewares.RequireVerified(), recipeHandler.Publish)
	recipeAuthor.PATCH("/:id/visibility", recipeHandler.Visibility)
	recipeAuthor.DELETE("/:id", recipeHandler.Delete)
	return r
}
