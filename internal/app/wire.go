package app

import (
	"github.com/wizzyszn/cooked/internal/auth"
	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/delicacy"
	"github.com/wizzyszn/cooked/internal/notify"
	"github.com/wizzyszn/cooked/internal/user"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Dependencies struct {
	Config          *config.Config
	AuthService     *auth.AuthService
	DelicacyService *delicacy.Service
	Tokens          *auth.JWTManager
	Notifier        *notify.AsyncNotifier
	Users           *user.Repository
	UserService     *user.Service
	GoogleService   *auth.GoogleService
	Database        *gorm.DB
	Logger          *zap.SugaredLogger
}

func NewDependencies(cfg *config.Config, db *gorm.DB, zapLogger *zap.SugaredLogger) *Dependencies {
	users := user.NewRepository(db)
	delicacyRepo := delicacy.NewRepository(db)
	authRepo := auth.NewRepository(db)
	tokens := auth.NewJWTManager(cfg.JWT)

	store := notify.NewStore(db)
	var providers []notify.ChannelProvider
	if cfg.Brevo.Enabled() {
		providers = append(providers, notify.NewBrevoEmailProvider(&cfg.Brevo, zapLogger))
	} else if zapLogger != nil {
		zapLogger.Warn("Brevo is not configured; email delivery is disabled")
	}
	notifier := notify.NewAsyncNotifier(store, users, providers, zapLogger)

	authService := auth.NewAuthService(&cfg.JWT, users, tokens, notifier, cfg.App.PublicURL, zapLogger, authRepo)
	userService := user.NewService(users, zapLogger)
	googleService := auth.NewGoogleService(cfg.GoogleOAuth, authRepo, authService)
	delicacyService := delicacy.NewDelicacyService(zapLogger, delicacyRepo)
	return &Dependencies{
		Config:          cfg,
		Database:        db,
		AuthService:     authService,
		DelicacyService: delicacyService,
		Tokens:          tokens,
		Notifier:        notifier,
		Users:           users,
		UserService:     userService,
		GoogleService:   googleService,
		Logger:          zapLogger,
	}
}
