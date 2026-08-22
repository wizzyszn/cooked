package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/domain"
	"github.com/wizzyszn/cooked/internal/notify"
	"github.com/wizzyszn/cooked/internal/user"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"go.uber.org/zap"
)

type AuthService struct {
	cfg       *config.JWTConfig
	users     *user.Repository
	authRepo  *Repository
	tokens    *JWTManager
	notifier  notify.Notifier
	publicURL string
	log       *zap.SugaredLogger
}

func NewAuthService(
	config *config.JWTConfig,
	users *user.Repository,
	tokens *JWTManager,
	notifier notify.Notifier,
	publicURL string,
	log *zap.SugaredLogger,
	authRepo *Repository,
) *AuthService {
	return &AuthService{
		cfg:       config,
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
		return nil, apperrors.Internal(s.log, "register email lookup", err)
	}
	if existingEmail != nil {
		return nil, apperrors.ErrEmailTaken
	}

	existingUsername, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		return nil, apperrors.Internal(s.log, "register username lookup", err)
	}
	if existingUsername != nil {
		return nil, apperrors.ErrUsernameTaken
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, apperrors.Internal(s.log, "hash password", err)
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
				return nil, apperrors.Internal(s.log, "register unique-violation lookup", lookupErr)
			}
			if taken != nil {
				return nil, apperrors.ErrEmailTaken
			}
			return nil, apperrors.ErrUsernameTaken
		}
		return nil, apperrors.Internal(s.log, "create user", err)
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
		return nil, apperrors.ErrInternalServerError.Wrap(err, apperrors.ErrInternalServerError.Code, apperrors.ErrInternalServerError.HTTPStatus)
	}
	if account == nil || !strings.EqualFold(account.Email, claims.Email) {
		return nil, apperrors.ErrInvalidToken
	}

	if !account.IsVerified {
		if err := s.users.MarkEmailVerified(ctx, account.ID); err != nil {
			return nil, apperrors.ErrInternalServerError.Wrap(err, apperrors.ErrInternalServerError.Code, apperrors.ErrInternalServerError.HTTPStatus)
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
		return nil, apperrors.Internal(s.log, "login lookup", err)
	}

	storedHash := ""
	if account != nil {
		storedHash = account.HashPass
	}
	if account == nil || !passwordMatches(storedHash, password) {
		return nil, apperrors.ErrInvalidEmailOrPassword
	}
	if !account.IsVerified {
		return nil, apperrors.ErrEmailNotVerified
	}

	accessToken, accessExp, err := s.tokens.IssueAccessToken(account.ID, account.Email, account.IsVerified)
	if err != nil {
		return nil, apperrors.Internal(s.log, "issue access token", err, "user_id", account.ID)
	}
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		if errors.Is(err, ErrReadByte) {
			return nil, apperrors.Internal(s.log, "generate refresh token", ErrReadByte, "user_id", account.ID)
		}
		return nil, apperrors.Internal(s.log, "generate refresh token", err, "user_id", account.ID)
	}

	if err := s.authRepo.CreateRefreshToken(ctx, &domain.RefreshToken{
		UserID:    account.ID,
		TokenHash: hash,
		FamilyID:  uuid.New(),
		ExpiresAt: time.Now().UTC().Add(time.Duration(s.cfg.RefreshTLLDay) * 24 * time.Hour),
		RevokedAt: nil,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, apperrors.Internal(s.log, "persist refresh token", err, "user_id", account.ID)
	}

	return &LoginResponse{
		User: account.Sanitize(),
		Tokens: &TokenPair{
			AccessToken:       accessToken,
			RefreshToken:      raw,
			AccessTokenExpiry: accessExp,
		},
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, req *RefreshRequest) (*RefreshResponse, error) {
	hashedRefreshToken := HashRefreshToken(req.RefreshToken)
	row, err := s.authRepo.FindRefreshTokenByHash(ctx, hashedRefreshToken)
	if err != nil {
		return nil, apperrors.Internal(s.log, "get refresh token", err)
	}
	if row == nil {
		return nil, apperrors.ErrInvalidToken
	}
	if row.RevokedAt != nil {
		n, err := s.authRepo.RevokeFamily(ctx, row.FamilyID)
		if err != nil {
			return nil, apperrors.Internal(s.log, "revoke token family", err, "user_id", row.UserID, "family_id", row.FamilyID)
		}
		if s.log != nil {
			s.log.Infow("refresh token reuse detected - family revoked", "user_id", row.UserID, "family_id", row.FamilyID, "revoked", n)
		}
		return nil, apperrors.ErrInvalidToken
	}
	if row.ExpiresAt.Before(time.Now()) {
		if err := s.authRepo.Revoke(ctx, row.ID); err != nil {
			return nil, apperrors.Internal(s.log, "revoke expired refresh token", err, "user_id", row.UserID)
		}
		return nil, apperrors.ErrInvalidToken
	}

	account, err := s.users.FindByID(ctx, row.UserID)
	if err != nil {
		return nil, apperrors.Internal(s.log, "account lookup by id", err, "user_id", row.UserID)
	}
	if account == nil {
		return nil, apperrors.ErrInvalidToken
	}

	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		if errors.Is(err, ErrReadByte) {
			return nil, apperrors.Internal(s.log, "generate refresh token", ErrReadByte, "user_id", account.ID)
		}
		return nil, apperrors.Internal(s.log, "generate refresh token", err, "user_id", account.ID)
	}
	newAccessToken, _, err := s.tokens.IssueAccessToken(account.ID, account.Email, account.IsVerified)
	if err != nil {
		return nil, apperrors.Internal(s.log, "issue access token", err, "user_id", account.ID)
	}

	newRow := &domain.RefreshToken{
		UserID:    account.ID,
		TokenHash: hash,
		FamilyID:  row.FamilyID,
		ParentID:  &row.ID,
		ExpiresAt: time.Now().UTC().Add(time.Duration(s.cfg.RefreshTLLDay) * 24 * time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.authRepo.RotateRefreshToken(ctx, row.ID, newRow); err != nil {
		if errors.Is(err, errRefreshReuse) {
			if s.log != nil {
				s.log.Infow("refresh token reuse detected - family revoked", "user_id", row.UserID, "family_id", row.FamilyID)
			}
			return nil, apperrors.ErrInvalidToken
		}
		if errors.Is(err, errRefreshExpired) || errors.Is(err, errRefreshNotFound) {
			return nil, apperrors.ErrInvalidToken
		}
		return nil, apperrors.Internal(s.log, "rotate refresh token", err, "user_id", account.ID)
	}

	return &RefreshResponse{
		AccessToken:  newAccessToken,
		RefreshToken: raw,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, req *LogoutRequest) error {
	hashedToken := HashRefreshToken(req.RefreshToken)
	row, err := s.authRepo.FindRefreshTokenByHash(ctx, hashedToken)
	if err != nil {
		return apperrors.Internal(s.log, "get refresh token", err)
	}
	if row == nil {
		return apperrors.ErrInvalidToken
	}
	if err := s.authRepo.Revoke(ctx, row.ID); err != nil {
		return apperrors.Internal(s.log, "revoke refresh token", err, "token_id", row.ID)
	}
	return nil
}

// LogoutAll revokes every refresh token in the same family as the presented token.
func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID, req *LogoutRequest) error {
	hashedToken := HashRefreshToken(req.RefreshToken)
	row, err := s.authRepo.FindRefreshTokenByHash(ctx, hashedToken)
	if err != nil {
		return apperrors.Internal(s.log, "get refresh token", err)
	}
	if row == nil || row.UserID != userID {
		return apperrors.ErrInvalidToken
	}
	if _, err := s.authRepo.RevokeFamily(ctx, row.FamilyID); err != nil {
		return apperrors.Internal(s.log, "revoke family tokens", err, "user_id", userID, "family_id", row.FamilyID)
	}
	return nil
}

func (s *AuthService) ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) error {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	account, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return apperrors.Internal(s.log, "account lookup", err, "email", email)
	}
	// Always succeed to avoid email enumeration; only send when the account exists.
	if account == nil {
		return nil
	}

	otp, err := generateOtp()
	if err != nil {
		return apperrors.Internal(s.log, "generate OTP", err)
	}
	if err := s.authRepo.InvalidateUnusedPasswordResetTokens(ctx, account.ID); err != nil {
		return apperrors.Internal(s.log, "invalidate prior password reset tokens", err, "user_id", account.ID)
	}
	row := domain.PasswordResetToken{
		UserID:    account.ID,
		Code:      otp,
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	}
	if err := s.authRepo.CreatePasswordResetToken(ctx, &row); err != nil {
		return apperrors.Internal(s.log, "create password reset token", err, "email", email)
	}
	if s.notifier == nil {
		return apperrors.Internal(s.log, "enqueue password reset otp", errors.New("notifier not configured"), "user_id", account.ID)
	}
	if err := s.notifier.Notify(ctx, notify.NotificationRequest{
		UserID:   account.ID,
		Channel:  domain.NotificationChannelEmail,
		Template: notify.TemplateForgotPassOtp,
		Payload: map[string]any{
			"name":  account.Name,
			"email": account.Email,
			"otp":   otp,
		},
	}); err != nil {
		return apperrors.Internal(s.log, "enqueue password reset otp", err, "user_id", account.ID)
	}
	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, req *ResetPasswordRequest) error {
	otp := strings.TrimSpace(req.Otp)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	newPassword := req.Password

	account, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return apperrors.Internal(s.log, "account lookup", err, "email", email)
	}
	if account == nil {
		return apperrors.New("INVALID_RESET", "Invalid or expired reset code", http.StatusBadRequest)
	}
	row, err := s.authRepo.GetPasswordResetToken(ctx, otp, account.ID)
	if err != nil {
		return apperrors.Internal(s.log, "get password reset token", err, "user_id", account.ID, "email", email)
	}
	if row == nil {
		return apperrors.New("INVALID_RESET", "Invalid or expired reset code", http.StatusBadRequest)
	}
	if row.UsedAt != nil {
		return apperrors.New("USED_OTP", "This reset code has already been used", http.StatusBadRequest)
	}
	if time.Now().UTC().After(row.ExpiresAt) {
		return apperrors.New("RESET_OTP_EXPIRED", "Reset code has expired. Request a new one and try again", http.StatusBadRequest)
	}
	hashedPass, err := HashPassword(newPassword)
	if err != nil {
		return apperrors.Internal(s.log, "hash password", err, "user_id", account.ID, "email", account.Email)
	}
	account.HashPass = hashedPass
	if err := s.users.Update(ctx, account); err != nil {
		return apperrors.Internal(s.log, "persist new password", err, "user_id", account.ID, "email", account.Email)
	}
	if err := s.authRepo.MarkPasswordResetTokenAsUsed(ctx, row.ID); err != nil {
		return apperrors.Internal(s.log, "mark used reset token", err, "user_id", account.ID, "email", account.Email)
	}
	// Password change invalidates all existing sessions.
	if _, err := s.authRepo.RevokeAllForUser(ctx, account.ID); err != nil {
		return apperrors.Internal(s.log, "revoke sessions after password reset", err, "user_id", account.ID)
	}
	return nil
}

func generateOtp() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n), nil
}
