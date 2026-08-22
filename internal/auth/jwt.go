package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/config"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
)

const purposeEmailVerify = "email_verify"

type AccessClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email"`
	UserID        string `json:"user_id"`
	EmailVerified bool   `json:"email_verified"`
}

type EmailVerifyClaims struct {
	jwt.RegisteredClaims
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
}

type EmailVerification struct {
	UserID uuid.UUID
	Email  string
}

type TokenPair struct {
	AccessToken       string    `json:"access_token"`
	RefreshToken      string    `json:"refresh_token"`
	AccessTokenExpiry time.Time `json:"access_token_expiry"`
}

type JWTManager struct {
	accessSecret   string
	refreshSecret  string
	accessTTL      time.Duration
	refreshTTL     time.Duration
	emailVerifyTTL time.Duration
}

func NewJWTManager(cfg config.JWTConfig) *JWTManager {
	emailTTL := time.Duration(cfg.EmailVerifyTTLHours) * time.Hour
	if emailTTL <= 0 {
		emailTTL = 24 * time.Hour
	}
	return &JWTManager{
		accessSecret:   cfg.AccessSecret,
		refreshSecret:  cfg.RefreshSecret,
		accessTTL:      time.Duration(cfg.AccessTTLMin) * time.Minute,
		refreshTTL:     time.Duration(cfg.RefreshTLLDay) * 24 * time.Hour,
		emailVerifyTTL: emailTTL,
	}
}

func (m *JWTManager) AccessTTL() time.Duration      { return m.accessTTL }
func (m *JWTManager) RefreshTTL() time.Duration     { return m.refreshTTL }
func (m *JWTManager) EmailVerifyTTL() time.Duration { return m.emailVerifyTTL }

func (m *JWTManager) accessKey() []byte { return []byte(m.accessSecret) }

func (m *JWTManager) IssueAccessToken(userId uuid.UUID, email string, isVerified bool) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(m.accessTTL)
	claims := AccessClaims{
		Email:         email,
		UserID:        userId.String(),
		EmailVerified: isVerified,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userId.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.accessKey())
	if err != nil {
		return "", time.Time{}, apperrors.ErrInternalServerError
	}
	return signedToken, exp, nil
}

func (m *JWTManager) IssueEmailVerificationToken(userID uuid.UUID, email string) (string, error) {
	now := time.Now().UTC()
	claims := EmailVerifyClaims{
		Email:   email,
		Purpose: purposeEmailVerify,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.emailVerifyTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.accessKey())
	if err != nil {
		return "", apperrors.ErrInternalServerError
	}
	return signed, nil
}

func (m *JWTManager) ParseEmailVerificationToken(raw string) (*EmailVerification, error) {
	claims := &EmailVerifyClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperrors.ErrInvalidToken
		}
		return m.accessKey(), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, apperrors.ErrTokenExpired
		}
		return nil, apperrors.ErrInvalidToken
	}
	if token == nil || !token.Valid || claims.Purpose != purposeEmailVerify {
		return nil, apperrors.ErrInvalidToken
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, apperrors.ErrInvalidToken
	}
	return &EmailVerification{UserID: userID, Email: claims.Email}, nil
}

func (m *JWTManager) Parse(raw string) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(raw, &AccessClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperrors.ErrInvalidToken
		}
		return m.accessKey(), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, apperrors.ErrTokenExpired
		}
		return nil, apperrors.ErrInvalidToken
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, apperrors.ErrInvalidToken
	}
	return claims, nil
}
