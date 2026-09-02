package db

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/auth"
	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/health"
	"github.com/wizzyszn/cooked/internal/user"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMigrationLifecycle(t *testing.T) {
	rawURL := os.Getenv("COOKED_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("COOKED_TEST_DATABASE_URL is not configured")
	}

	base, err := gorm.Open(postgres.Open(rawURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	var databaseName string
	if err := base.Raw("SELECT current_database()").Scan(&databaseName).Error; err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if err := validateMigrationTestTarget(databaseName, os.Getenv("COOKED_TEST_ALLOW_SHARED_DATABASE")); err != nil {
		t.Fatal(err)
	}

	schema := "cooked_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		if err := base.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
	})

	scopedURL, err := withSearchPath(rawURL, schema)
	if err != nil {
		t.Fatalf("scope database URL: %v", err)
	}
	database, err := gorm.Open(postgres.Open(scopedURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to isolated schema: %v", err)
	}

	if err := MigrateToVersion(database, 6); err != nil {
		t.Fatalf("migrate empty schema to v6 fixture: %v", err)
	}
	assertMigrationVersion(t, database, 6)

	if err := MigrateUp(database); err != nil {
		t.Fatalf("migrate v6 fixture to latest: %v", err)
	}
	assertMigrationVersion(t, database, LatestMigrationVersion)
	assertRegisteredRole(t, database)
	assertReadiness(t, database)
	assertTransactionRollback(t, database)
	assertM1IdentityLifecycle(t, database)

	if err := MigrateSteps(database, -1); err != nil {
		t.Fatalf("roll back latest migration: %v", err)
	}
	assertMigrationVersion(t, database, LatestMigrationVersion-1)

	if err := MigrateUp(database); err != nil {
		t.Fatalf("reapply latest migration: %v", err)
	}
	assertMigrationVersion(t, database, LatestMigrationVersion)

	if err := MigrateDown(database); err != nil {
		t.Fatalf("roll back all migrations: %v", err)
	}
	if _, _, err := CurrentVersion(database); !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("expected nil migration version after full rollback, got %v", err)
	}

	if err := MigrateUp(database); err != nil {
		t.Fatalf("migrate empty schema to latest: %v", err)
	}
	assertMigrationVersion(t, database, LatestMigrationVersion)
}

func assertM1IdentityLifecycle(t *testing.T, database *gorm.DB) {
	t.Helper()
	accountID, delicacyID, recipeID := uuid.New(), uuid.New(), uuid.New()
	if err := database.Exec("INSERT INTO users (id,email,name,user_name,is_verified,hash_pass) VALUES (?,?,?,?,true,'hash')", accountID, "m1@example.com", "M One", "m_one").Error; err != nil {
		t.Fatalf("insert M1 user: %v", err)
	}
	if err := database.Exec("INSERT INTO delicacies (id,name,description,created_by) VALUES (?,?,?,?)", delicacyID, "M1 dish", "test", accountID).Error; err != nil {
		t.Fatalf("insert M1 delicacy: %v", err)
	}
	if err := database.Exec("INSERT INTO recipes (id,user_id,delicacy_id,title,algo) VALUES (?,?,?,?,?)", recipeID, accountID, delicacyID, "M1 recipe", "test").Error; err != nil {
		t.Fatalf("insert retained recipe: %v", err)
	}
	if err := database.Exec("INSERT INTO refresh_tokens (id,user_id,token_hash,family_id,expires_at,created_at) VALUES (?,?,?,?,?,?)", uuid.New(), accountID, strings.Repeat("a", 64), uuid.New(), time.Now().Add(time.Hour), time.Now()).Error; err != nil {
		t.Fatalf("insert refresh token: %v", err)
	}
	if err := database.Exec("INSERT INTO oauth_identities (user_id,provider,provider_subject,email) VALUES (?,'google','subject-1','m1@example.com')", accountID).Error; err != nil {
		t.Fatalf("insert oauth identity: %v", err)
	}
	repo := user.NewRepository(database)
	assertGoogleOAuthLifecycle(t, database, repo)
	if err := repo.ReplaceDietaryPreferences(t.Context(), accountID, []string{"vegan", "halal"}); err != nil {
		t.Fatalf("replace dietary preferences: %v", err)
	}
	loaded, err := repo.FindByID(t.Context(), accountID)
	if err != nil || len(loaded.DietaryPreferences) != 2 {
		t.Fatalf("dietary round trip: preferences=%v err=%v", loaded.DietaryPreferences, err)
	}
	if err := repo.Anonymize(t.Context(), accountID, time.Now().UTC()); err != nil {
		t.Fatalf("anonymize account: %v", err)
	}
	var liveTokens, identities, retainedRecipes, audits int64
	database.Raw("SELECT count(*) FROM refresh_tokens WHERE user_id=? AND revoked_at IS NULL", accountID).Scan(&liveTokens)
	database.Raw("SELECT count(*) FROM oauth_identities WHERE user_id=?", accountID).Scan(&identities)
	database.Raw("SELECT count(*) FROM recipes WHERE id=? AND user_id=?", recipeID, accountID).Scan(&retainedRecipes)
	database.Raw("SELECT count(*) FROM audit_logs WHERE target_id=? AND action='user.anonymized'", accountID).Scan(&audits)
	loaded, err = repo.FindByID(t.Context(), accountID)
	if err != nil || loaded == nil || loaded.DeactivatedAt == nil || loaded.Name != "Deleted user" || strings.Contains(loaded.Email, "m1@") {
		t.Fatalf("anonymized user invalid: %#v err=%v", loaded, err)
	}
	if liveTokens != 0 || identities != 0 || retainedRecipes != 1 || audits != 1 {
		t.Fatalf("anonymization invariant tokens=%d identities=%d recipes=%d audits=%d", liveTokens, identities, retainedRecipes, audits)
	}
}

type fakeGoogleProvider struct{ state, nonce string }

func (p *fakeGoogleProvider) AuthorizationURL(state, nonce, challenge string) string {
	p.state, p.nonce = state, nonce
	return "https://accounts.example/authorize?state=" + url.QueryEscape(state)
}
func (p *fakeGoogleProvider) ExchangeIdentity(_ context.Context, code, verifier, expectedNonceHash string) (*auth.GoogleIdentity, error) {
	sum := sha256.Sum256([]byte(p.nonce))
	if code != "provider-code" || verifier == "" || fmt.Sprintf("%x", sum[:]) != expectedNonceHash {
		return nil, errors.New("invalid provider exchange")
	}
	return &auth.GoogleIdentity{Subject: "google-subject", Email: "google-user@example.com", EmailVerified: true, Name: "Google User"}, nil
}

func assertGoogleOAuthLifecycle(t *testing.T, database *gorm.DB, users *user.Repository) {
	t.Helper()
	jwtCfg := config.JWTConfig{AccessSecret: "test-access-secret-that-is-long-enough", RefreshSecret: "test-refresh-secret-that-is-long-enough", AccessTTLMin: 15, RefreshTLLDay: 14}
	tokens := auth.NewJWTManager(jwtCfg)
	authRepo := auth.NewRepository(database)
	authService := auth.NewAuthService(&jwtCfg, users, tokens, nil, "http://localhost", nil, authRepo)
	provider := &fakeGoogleProvider{}
	googleCfg := config.GoogleOAuthConfig{ClientID: "client", ClientSecret: "secret", RedirectURL: "http://api.example/callback", AllowedReturnURLs: []string{"https://app.example/auth/callback"}}
	service := auth.NewGoogleServiceWithProvider(googleCfg, provider, authRepo, authService)
	start, err := service.Start(t.Context(), "https://app.example/auth/callback")
	if err != nil {
		t.Fatalf("start google oauth: %v", err)
	}
	parsed, _ := url.Parse(start.AuthorizationURL)
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatal("oauth state was not returned")
	}
	location, err := service.Callback(t.Context(), state, "provider-code")
	if err != nil {
		t.Fatalf("google callback: %v", err)
	}
	redirect, _ := url.Parse(location)
	loginCode := redirect.Query().Get("code")
	if loginCode == "" {
		t.Fatal("one-time login code was not returned")
	}
	login, err := service.Exchange(t.Context(), loginCode)
	if err != nil {
		t.Fatalf("exchange google code: %v", err)
	}
	if login.User.Email != "google-user@example.com" || login.Tokens.AccessToken == "" || login.Tokens.RefreshToken == "" {
		t.Fatalf("incomplete google login response: %#v", login)
	}
	if _, err := service.Exchange(t.Context(), loginCode); err == nil {
		t.Fatal("one-time google login code was reusable")
	}
}

func withSearchPath(rawURL, schema string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse database URL: %w", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func assertMigrationVersion(t *testing.T, database *gorm.DB, want uint) {
	t.Helper()
	got, dirty, err := CurrentVersion(database)
	if err != nil {
		t.Fatalf("read migration version: %v", err)
	}
	if got != want || dirty {
		t.Fatalf("migration state = (%d, dirty=%v), want (%d, false)", got, dirty, want)
	}
}

func assertRegisteredRole(t *testing.T, database *gorm.DB) {
	t.Helper()
	userID := uuid.New()
	err := database.Exec(
		"INSERT INTO users (id, email, name, user_name, is_verified) VALUES (?, ?, ?, ?, false)",
		userID,
		"role-test@example.com",
		"Role Test",
		"role_test",
	).Error
	if err != nil {
		t.Fatalf("insert user after role migration: %v", err)
	}
	var count int64
	if err := database.Raw("SELECT count(*) FROM user_roles WHERE user_id = ? AND role = 'user'", userID).Scan(&count).Error; err != nil {
		t.Fatalf("query registered role: %v", err)
	}
	if count != 1 {
		t.Fatalf("registered role count = %d, want 1", count)
	}
}

func assertReadiness(t *testing.T, database *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health/ready", health.NewHandler(database, LatestMigrationVersion).Ready)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func assertTransactionRollback(t *testing.T, database *gorm.DB) {
	t.Helper()
	sentinel := errors.New("force rollback")
	userID := uuid.New()
	err := WithinTransaction(t.Context(), database, func(tx *gorm.DB) error {
		if err := tx.Exec(
			"INSERT INTO users (id, email, name, user_name, is_verified) VALUES (?, ?, ?, ?, false)",
			userID,
			"rollback-test@example.com",
			"Rollback Test",
			"rollback_test",
		).Error; err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("transaction error = %v, want sentinel", err)
	}
	var count int64
	if err := database.Raw("SELECT count(*) FROM users WHERE id = ?", userID).Scan(&count).Error; err != nil {
		t.Fatalf("query rolled-back user: %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled-back user count = %d, want 0", count)
	}
}

func TestWithSearchPath(t *testing.T) {
	got, err := withSearchPath("postgres://user:pass@localhost/cooked_test?sslmode=disable", "isolated")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("search_path") != "isolated" {
		t.Fatalf("search_path = %q", parsed.Query().Get("search_path"))
	}
}

func validateMigrationTestTarget(databaseName, sharedOptIn string) error {
	if strings.HasSuffix(databaseName, "_test") || sharedOptIn == "1" {
		return nil
	}
	return fmt.Errorf("refusing migration lifecycle against non-test database %q without COOKED_TEST_ALLOW_SHARED_DATABASE=1", databaseName)
}

func TestValidateMigrationTestTarget(t *testing.T) {
	for _, tt := range []struct {
		name       string
		database   string
		optIn      string
		shouldFail bool
	}{
		{name: "dedicated test database", database: "cooked_test"},
		{name: "explicit isolated schema opt in", database: "cooked", optIn: "1"},
		{name: "reject ordinary database", database: "cooked", shouldFail: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMigrationTestTarget(tt.database, tt.optIn)
			if (err != nil) != tt.shouldFail {
				t.Fatalf("error = %v, shouldFail = %v", err, tt.shouldFail)
			}
		})
	}
}
