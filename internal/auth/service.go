package auth

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/notify"
	"github.com/wizzyszn/cooked/internal/user"
	"github.com/wizzyszn/cooked/pkg/errors"
	"go.uber.org/zap"
)

type AuthService struct {
	users     *user.Repository
	authRepo  *Repository
	tokens    *JWTManager
	notifier  notify.Notifier
	publicURL string
	log       *zap.SugaredLogger
}

func NewAuthService(
	users *user.Repository,
	tokens *JWTManager,
	notifier notify.Notifier,
	publicURL string,
	log *zap.SugaredLogger,
	authRepo *Repository,
) *AuthService {
	return &AuthService{
		users:     users,
		tokens:    tokens,
		notifier:  notifier,
		publicURL: strings.TrimRight(publicURL, "/"),
		log:       log,
		authRepo:  authRepo,
	}
}

func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	username := strings.TrimSpace(req.UserName)
	name := strings.TrimSpace(req.Name)

	existingEmail, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, errors.Internal(s.log, "register email lookup", err)
	}
	if existingEmail != nil {
		return nil, errors.ErrEmailTaken
	}

	existingUsername, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.Internal(s.log, "register username lookup", err)
	}
	if existingUsername != nil {
		return nil, errors.ErrUsernameTaken
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, errors.Internal(s.log, "hash password", err)
	}

	account := &domain.User{
		Email:      email,
		Name:       name,
		UserName:   username,
		IsVerified: false,
		HashPass:   hash,
	}
	if err := s.users.Create(ctx, account); err != nil {
		if user.IsUniqueViolation(err) {
			taken, lookupErr := s.users.FindByEmail(ctx, email)
			if lookupErr != nil {
				return nil, errors.Internal(s.log, "register unique-violation lookup", lookupErr)
			}
			if taken != nil {
				return nil, errors.ErrEmailTaken
			}
			return nil, errors.ErrUsernameTaken
		}
		return nil, errors.Internal(s.log, "create user", err)
	}

	s.enqueueVerificationEmail(ctx, account)

	return &RegisterResponse{
		User:    account.Sanitize(),
		Message: "account created; check your email to confirm your address",
	}, nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, rawToken string) (*VerifyEmailResponse, error) {
	claims, err := s.tokens.ParseEmailVerificationToken(rawToken)
	if err != nil {
		return nil, err
	}

	account, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.ErrInternalServerError.Wrap(err, errors.ErrInternalServerError.Code, errors.ErrInternalServerError.HTTPStatus)
	}
	if account == nil || !strings.EqualFold(account.Email, claims.Email) {
		return nil, errors.ErrInvalidToken
	}

	if !account.IsVerified {
		if err := s.users.MarkEmailVerified(ctx, account.ID); err != nil {
			return nil, errors.ErrInternalServerError.Wrap(err, errors.ErrInternalServerError.Code, errors.ErrInternalServerError.HTTPStatus)
		}
		account.IsVerified = true
	}

	return &VerifyEmailResponse{
		User:    account.Sanitize(),
		Message: "email confirmed",
	}, nil
}

func (s *AuthService) enqueueVerificationEmail(ctx context.Context, account *domain.User) {
	if s.notifier == nil || s.tokens == nil {
		return
	}

	token, err := s.tokens.IssueEmailVerificationToken(account.ID, account.Email)
	if err != nil {
		if s.log != nil {
			s.log.Errorw("issue email verification token", "user_id", account.ID, "error", err)
		}
		return
	}

	verifyURL := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", s.publicURL, url.QueryEscape(token))
	err = s.notifier.Notify(ctx, notify.NotificationRequest{
		UserID:   account.ID,
		Channel:  domain.NotificationChannelEmail,
		Template: notify.TemplateVerifyEmail,
		Payload: map[string]any{
			"name":       account.Name,
			"email":      account.Email,
			"verify_url": verifyURL,
		},
	})
	if err != nil && s.log != nil {
		s.log.Errorw("enqueue verification email", "user_id", account.ID, "error", err)
	}
}

func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	account, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, errors.Internal(s.log, "login lookup", err)
	}

	storedHash := ""
	if account != nil {
		storedHash = account.HashPass
	}
	if account == nil || !passwordMatches(storedHash, password) {
		return nil, errors.ErrInvalidEmailOrPassword
	}
	if !account.IsVerified {
		return nil, errors.ErrEmailNotVerified
	}

	accessToken, accessExp, err := s.tokens.IssueAccessToken(account.ID, account.Email, account.IsVerified)
	if err != nil {
		return nil, errors.Internal(s.log, "issue access token", err, "user_id", account.ID)

	}
	refreshToken, refreshExp, err := s.tokens.IssueRefreshToken(account.ID, account.Email, account.IsVerified)
	if err != nil {
		return nil, errors.Internal(s.log, "issue refresh token", err, "user_id", account.ID)
	}

	if err := s.authRepo.CreateRefreshToken(ctx, &domain.RefreshToken{
		UserID:    account.ID,
		TokenHash: HashRefreshToken(refreshToken),
		ExpiresAt: refreshExp,
		RevokedAt: nil,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, errors.Internal(s.log, "persist refresh token", err, "user_id", account.ID)
	}

	return &LoginResponse{
		User: account.Sanitize(),
		Tokens: &TokenPair{
			AccessToken:       accessToken,
			RefreshToken:      refreshToken,
			AccessTokenExpiry: accessExp,
		},
	}, nil
}
