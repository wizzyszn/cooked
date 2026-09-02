package app

import (
	"github.com/wizzyszn/cooked/internal/auth"
	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/delicacy"
	"github.com/wizzyszn/cooked/internal/media"
	"github.com/wizzyszn/cooked/internal/notify"
	"github.com/wizzyszn/cooked/internal/recipe"
	"github.com/wizzyszn/cooked/internal/user"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Dependencies struct {
	Config          *config.Config
	AuthService     *auth.AuthService
	DelicacyService *delicacy.Service
	Tokens          *auth.JWTManager
	Notifier        notify.Notifier
	MediaService    *media.Service
	ObjectStore     media.ObjectStore
	Users           *user.Repository
	UserService     *user.Service
	GoogleService   *auth.GoogleService
	RecipeService   *recipe.Service
	Database        *gorm.DB
	Logger          *zap.SugaredLogger
}

func NewDependencies(cfg *config.Config, db *gorm.DB, zapLogger *zap.SugaredLogger) *Dependencies {
	users := user.NewRepository(db)
	delicacyRepo := delicacy.NewRepository(db)
	authRepo := auth.NewRepository(db)
	tokens := auth.NewJWTManager(cfg.JWT)

	store := notify.NewStore(db)
	notifier := notify.NewOutboxNotifier(store, zapLogger)
	var objectStore media.ObjectStore
	var mediaService *media.Service
	if cfg.ObjectStorage.Enabled() {
		s3, err := media.NewS3Store(cfg.ObjectStorage)
		if err != nil {
			if zapLogger != nil {
				zapLogger.Errorw("initialize object storage", "error", err)
			}
		} else {
			objectStore = s3
			mediaService = media.NewService(media.NewRepository(db), s3, zapLogger)
		}
	} else if zapLogger != nil {
		zapLogger.Warn("S3-compatible object storage is not configured; media endpoints are disabled")
	}

	authService := auth.NewAuthService(&cfg.JWT, users, tokens, notifier, cfg.App.PublicURL, zapLogger, authRepo)
	userService := user.NewService(users, zapLogger)
	if mediaService != nil {
		userService = user.NewServiceWithAvatars(users, mediaService, zapLogger)
	}
	googleService := auth.NewGoogleService(cfg.GoogleOAuth, authRepo, authService)
	delicacyService := delicacy.NewDelicacyService(zapLogger, delicacyRepo)
	recipeService := recipe.NewService(recipe.NewRepository(db))
	return &Dependencies{
		Config:          cfg,
		Database:        db,
		AuthService:     authService,
		DelicacyService: delicacyService,
		Tokens:          tokens,
		Notifier:        notifier,
		MediaService:    mediaService,
		ObjectStore:     objectStore,
		Users:           users,
		UserService:     userService,
		GoogleService:   googleService,
		RecipeService:   recipeService,
		Logger:          zapLogger,
	}
}
