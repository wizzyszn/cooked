package router

import (
	"github.com/gin-gonic/gin"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"github.com/wizzyszn/cooked/internal/app"
	"github.com/wizzyszn/cooked/internal/auth"
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
	r.Use(middlewares.RequestLogger(deps.Logger))
	r.Use(middlewares.NewRateLimiter(100).Limit)

	v1 := r.Group("/api/v1")
	authG := v1.Group("/auth")
	authHandler := auth.NewAuthHandler(deps.AuthService)

	reqAuthM := middlewares.RequireAuth(deps.Tokens)
	
	{
		authG.POST("/register", authHandler.Register)
		authG.GET("/verify-email", authHandler.VerifyEmail)
		authG.POST("/login", middlewares.NewRateLimiter(5).Limit, authHandler.Login)
		authG.POST("/refresh", authHandler.Refresh)
		authG.POST("/logout", authHandler.Logout)
		authG.POST("/forgot-password", middlewares.NewRateLimiter(5).Limit, authHandler.ForgotPassword)
		authG.POST("/reset-password", middlewares.NewRateLimiter(5).Limit, authHandler.ResetPassword)
	}
	authed := authG.Use(reqAuthM)
	{
		authed.POST("/logout-all", authHandler.LogoutAll)
	}

	return r
}
