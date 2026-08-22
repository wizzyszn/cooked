package auth

import (
	"testing"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/config"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
)

func testManager(t *testing.T) *JWTManager {
	t.Helper()
	return NewJWTManager(config.JWTConfig{
		AccessSecret:        "access-secret-for-tests",
		RefreshSecret:       "refresh-secret-for-tests",
		AccessTTLMin:        15,
		RefreshTLLDay:       14,
		EmailVerifyTTLHours: 24,
	})
}

func TestEmailVerificationTokenRoundTrip(t *testing.T) {
	m := testManager(t)
	userID := uuid.New()
	token, err := m.IssueEmailVerificationToken(userID, "ada@example.com")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, err := m.ParseEmailVerificationToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.UserID != userID || got.Email != "ada@example.com" {
		t.Fatalf("got %+v", got)
	}
}

func TestEmailVerificationTokenRejectsGarbage(t *testing.T) {
	m := testManager(t)
	if _, err := m.ParseEmailVerificationToken("not-a-token"); err != apperrors.ErrInvalidToken {
		t.Fatalf("expected invalid token, got %v", err)
	}
}

func TestGenerateRefreshTokenAreUnique(t *testing.T) {
	a, ha, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate a: %v", err)
	}
	b, hb, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate b: %v", err)
	}
	if a == b || ha == hb {
		t.Fatal("refresh tokens minted back-to-back must differ")
	}
	if HashRefreshToken(a) != ha {
		t.Fatal("returned hash must match HashRefreshToken(raw)")
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	m := testManager(t)
	userID := uuid.New()
	token, _, err := m.IssueAccessToken(userID, "ada@example.com", true)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != userID.String() || claims.Email != "ada@example.com" || !claims.EmailVerified {
		t.Fatalf("got %+v", claims)
	}
}

func TestAccessTokenRejectsGarbage(t *testing.T) {
	m := testManager(t)
	if _, err := m.Parse("not-a-token"); err != apperrors.ErrInvalidToken {
		t.Fatalf("expected invalid token, got %v", err)
	}
}
