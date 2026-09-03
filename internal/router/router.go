package router

import (
	"github.com/gin-gonic/gin"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"github.com/wizzyszn/cooked/internal/app"
	"github.com/wizzyszn/cooked/internal/auth"
	"github.com/wizzyszn/cooked/internal/cook"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/delicacy"
	"github.com/wizzyszn/cooked/internal/discovery"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/engagement"
	"github.com/wizzyszn/cooked/internal/health"
	"github.com/wizzyszn/cooked/internal/media"
	"github.com/wizzyszn/cooked/internal/observability"
	"github.com/wizzyszn/cooked/internal/recipe"
	"github.com/wizzyszn/cooked/internal/review"
	"github.com/wizzyszn/cooked/internal/swagger"
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
	metrics := observability.NewRegistry()
	r.Use(metrics.Middleware())
	r.Use(middlewares.RequestLogger(deps.Logger))

	healthHandler := health.NewHandler(deps.Database, db.LatestMigrationVersion)
	r.GET("/health/live", healthHandler.Live)
	r.GET("/health/ready", healthHandler.Ready)
	r.GET("/metrics", metrics.Handler(deps.Database))
	swagger.Register(r)

	// Operational and documentation routes stay reachable when an API client
	// exhausts its shared budget. Requests made through Swagger's "Try it out"
	// still target /api/v1 and therefore pass through this limiter.
	v1 := r.Group("/api/v1", middlewares.NewSharedRateLimiter(deps.Database, "global", deps.Config.RateLimits.Global, false).Limit)
	authG := v1.Group("/auth")
	authHandler := auth.NewAuthHandler(deps.AuthService)
	googleHandler := auth.NewGoogleHandler(deps.GoogleService)

	reqAuthM := middlewares.RequireAuth(deps.Tokens, deps.Users)
	//onboarding
	{
		authG.POST("/register", authHandler.Register)
		authG.GET("/verify-email", authHandler.VerifyEmail)
		authG.POST("/login", middlewares.NewSharedRateLimiter(deps.Database, "auth.login", deps.Config.RateLimits.Auth, false).Limit, authHandler.Login)
		authG.POST("/refresh", authHandler.Refresh)
		authG.POST("/logout", authHandler.Logout)
		authG.POST("/forgot-password", middlewares.NewSharedRateLimiter(deps.Database, "auth.forgot", deps.Config.RateLimits.Auth, false).Limit, authHandler.ForgotPassword)
		authG.POST("/reset-password", middlewares.NewSharedRateLimiter(deps.Database, "auth.reset", deps.Config.RateLimits.Auth, false).Limit, authHandler.ResetPassword)
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
		authed.POST("/media/uploads", middlewares.NewSharedRateLimiter(deps.Database, "media.upload", deps.Config.RateLimits.MediaUpload, true).Limit, mediaHandler.Initialize)
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
		verified.POST("", middlewares.NewSharedRateLimiter(deps.Database, "dish.submit", deps.Config.RateLimits.DishSubmission, true).Limit, delicacyHandler.Create(false))
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
	recipeAuthor.POST("/:id/publish", middlewares.RequireVerified(), middlewares.NewSharedRateLimiter(deps.Database, "recipe.publish", deps.Config.RateLimits.RecipePublication, true).Limit, recipeHandler.Publish)
	recipeAuthor.PATCH("/:id/visibility", recipeHandler.Visibility)
	recipeAuthor.DELETE("/:id", recipeHandler.Delete)
	discoveryHandler := discovery.NewHandler(deps.DiscoveryService)
	v1.GET("/search", discoveryHandler.Search)
	v1.GET("/browse/dishes", discoveryHandler.Browse)
	v1.GET("/discovery/recent-dishes", discoveryHandler.Recent)
	v1.GET("/discovery/trending", discoveryHandler.Trending)
	authed.GET("/discovery/recommendations", discoveryHandler.Recommendations)
	authed.GET("/users/me/favorites", discoveryHandler.Favorites)
	authed.PUT("/recipes/:id/favorite", discoveryHandler.Save)
	authed.DELETE("/recipes/:id/favorite", discoveryHandler.Unsave)
	notificationHandler := engagement.NewNotificationHandler(deps.NotificationService)
	authed.GET("/users/me/notification-preferences", notificationHandler.Preferences)
	authed.PUT("/users/me/notification-preferences", notificationHandler.SetPreference)
	authed.GET("/users/me/notifications", notificationHandler.Inbox)
	authed.POST("/users/me/notifications/:id/read", notificationHandler.MarkRead)
	cookHandler := cook.NewHandler(deps.CookService)
	v1.POST("/analytics/events", middlewares.OptionalAuth(deps.Tokens, deps.Users), cookHandler.Ingest)
	authed.POST("/cook-sessions", cookHandler.Start)
	authed.GET("/cook-sessions", cookHandler.List)
	authed.GET("/cook-sessions/active", cookHandler.Active)
	authed.GET("/cook-sessions/:id", cookHandler.Get)
	authed.POST("/cook-sessions/:id/steps/:stepId/visit", cookHandler.Visit)
	authed.PUT("/cook-sessions/:id/steps/:stepId/timer", cookHandler.Timer)
	authed.POST("/cook-sessions/:id/abandon", cookHandler.Abandon)
	authed.POST("/cook-sessions/:id/complete", middlewares.NewSharedRateLimiter(deps.Database, "cook.complete", deps.Config.RateLimits.CookCompletion, true).Limit, cookHandler.Complete)
	staff.GET("/metrics/product", cookHandler.Metrics)
	reviewHandler := review.NewHandler(deps.ReviewService)
	v1.GET("/reviews/:id", middlewares.OptionalAuth(deps.Tokens, deps.Users), reviewHandler.Get)
	v1.GET("/recipe-versions/:id/reviews", middlewares.OptionalAuth(deps.Tokens, deps.Users), reviewHandler.List)
	verifiedReviews := v1.Group("", reqAuthM, middlewares.RequireVerified())
	verifiedReviews.POST("/recipe-versions/:id/reviews", middlewares.NewSharedRateLimiter(deps.Database, "review.create", deps.Config.RateLimits.Review, true).Limit, reviewHandler.Create)
	verifiedReviews.PATCH("/reviews/:id", reviewHandler.Edit)
	verifiedReviews.POST("/reports", middlewares.NewSharedRateLimiter(deps.Database, "report.create", deps.Config.RateLimits.Report, true).Limit, reviewHandler.Report)
	staff.GET("/reports", reviewHandler.Queue)
	staff.POST("/reports/:id/decision", reviewHandler.Moderate)
	return r
}
