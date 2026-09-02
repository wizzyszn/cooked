package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/config"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
	"golang.org/x/oauth2"
)

type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}
type GoogleProvider interface {
	AuthorizationURL(state, nonce, challenge string) string
	ExchangeIdentity(context.Context, string, string, string) (*GoogleIdentity, error)
}
type googleProvider struct {
	cfg      *oauth2.Config
	clientID string
	client   *http.Client
}

func newGoogleProvider(cfg config.GoogleOAuthConfig) GoogleProvider {
	return &googleProvider{cfg: &oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectURL, Scopes: []string{"openid", "email", "profile"}, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token"}}, clientID: cfg.ClientID, client: http.DefaultClient}
}
func (p *googleProvider) AuthorizationURL(state, nonce, challenge string) string {
	return p.cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("nonce", nonce), oauth2.SetAuthURLParam("code_challenge", challenge), oauth2.SetAuthURLParam("code_challenge_method", "S256"))
}
func (p *googleProvider) ExchangeIdentity(ctx context.Context, code, verifier, expectedNonce string) (*GoogleIdentity, error) {
	token, err := p.cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return nil, err
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return nil, fmt.Errorf("google response omitted id_token")
	}
	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(raw)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google rejected id_token")
	}
	var claims struct {
		Sub           string `json:"sub"`
		Aud           string `json:"aud"`
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		Exp           string `json:"exp"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, err
	}
	if claims.Aud != p.clientID || claims.Sub == "" || claims.Email == "" {
		return nil, fmt.Errorf("invalid google token claims")
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var nonceClaim struct {
		Nonce string `json:"nonce"`
	}
	if json.Unmarshal(payload, &nonceClaim) != nil || subtle.ConstantTimeCompare([]byte(sha(nonceClaim.Nonce)), []byte(expectedNonce)) != 1 {
		return nil, fmt.Errorf("invalid nonce")
	}
	verified := claims.EmailVerified == true || claims.EmailVerified == "true"
	if !verified {
		return nil, fmt.Errorf("google email is not verified")
	}
	return &GoogleIdentity{Subject: claims.Sub, Email: claims.Email, EmailVerified: true, Name: claims.Name, Picture: claims.Picture}, nil
}

type GoogleService struct {
	cfg      config.GoogleOAuthConfig
	provider GoogleProvider
	repo     *Repository
	auth     *AuthService
}

func NewGoogleService(cfg config.GoogleOAuthConfig, repo *Repository, auth *AuthService) *GoogleService {
	var provider GoogleProvider
	if cfg.Enabled() {
		provider = newGoogleProvider(cfg)
	}
	return &GoogleService{cfg: cfg, provider: provider, repo: repo, auth: auth}
}
func NewGoogleServiceWithProvider(cfg config.GoogleOAuthConfig, provider GoogleProvider, repo *Repository, auth *AuthService) *GoogleService {
	return &GoogleService{cfg: cfg, provider: provider, repo: repo, auth: auth}
}

type GoogleStartRequest struct {
	ReturnURL string `json:"return_url" binding:"required,url,max=1024"`
}
type GoogleStartResponse struct {
	AuthorizationURL string `json:"authorization_url"`
}
type GoogleExchangeRequest struct {
	Code string `json:"code" binding:"required,min=32"`
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := randRead(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

var randRead = func(b []byte) (int, error) { return rand.Read(b) }

func sha(raw string) string { sum := sha256.Sum256([]byte(raw)); return fmt.Sprintf("%x", sum[:]) }
func (s *GoogleService) Start(ctx context.Context, returnURL string) (*GoogleStartResponse, error) {
	if s.provider == nil {
		return nil, apperrors.ErrServiceUnavailable
	}
	if !allowedReturnURL(returnURL, s.cfg.AllowedReturnURLs) {
		return nil, apperrors.ErrValidation
	}
	state, e := randomURLSafe(32)
	if e != nil {
		return nil, e
	}
	verifier, e := randomURLSafe(48)
	if e != nil {
		return nil, e
	}
	nonce, e := randomURLSafe(32)
	if e != nil {
		return nil, e
	}
	challengeBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])
	if e = s.repo.CreateOAuthFlow(ctx, &OAuthFlow{StateHash: sha(state), CodeVerifier: verifier, NonceHash: sha(nonce), ReturnURL: returnURL, ExpiresAt: time.Now().UTC().Add(10 * time.Minute)}); e != nil {
		return nil, apperrors.Internal(s.auth.log, "persist oauth flow", e)
	}
	return &GoogleStartResponse{AuthorizationURL: s.provider.AuthorizationURL(state, nonce, challenge)}, nil
}
func (s *GoogleService) Callback(ctx context.Context, state, code string) (string, error) {
	if s.provider == nil {
		return "", apperrors.ErrServiceUnavailable
	}
	flow, e := s.repo.ConsumeOAuthFlow(ctx, sha(state), time.Now().UTC())
	if e != nil {
		return "", apperrors.ErrInvalidToken
	}
	identity, e := s.provider.ExchangeIdentity(ctx, code, flow.CodeVerifier, flow.NonceHash)
	if e != nil {
		return "", apperrors.ErrInvalidToken
	}
	if identity == nil || !identity.EmailVerified {
		return "", apperrors.ErrInvalidToken
	}
	username := "user_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	account, e := s.repo.ResolveGoogleUser(ctx, identity.Subject, identity.Email, identity.Name, identity.Picture, username, time.Now().UTC())
	if e != nil {
		return "", apperrors.Internal(s.auth.log, "resolve google account", e)
	}
	raw, e := randomURLSafe(32)
	if e != nil {
		return "", apperrors.Internal(s.auth.log, "generate oauth login code", e)
	}
	if e = s.repo.CreateOAuthLoginCode(ctx, &OAuthLoginCode{CodeHash: sha(raw), UserID: account.ID, ExpiresAt: time.Now().UTC().Add(2 * time.Minute)}); e != nil {
		return "", apperrors.Internal(s.auth.log, "persist oauth login code", e)
	}
	return appendQuery(flow.ReturnURL, "code", raw), nil
}
func (s *GoogleService) Exchange(ctx context.Context, raw string) (*LoginResponse, error) {
	row, e := s.repo.ConsumeOAuthLoginCode(ctx, sha(raw), time.Now().UTC())
	if e != nil {
		return nil, apperrors.ErrInvalidToken
	}
	account, e := s.auth.users.FindByID(ctx, row.UserID)
	if e != nil || account == nil || account.DeactivatedAt != nil {
		return nil, apperrors.ErrInvalidToken
	}
	return s.auth.issueSession(ctx, account)
}
func allowedReturnURL(raw string, allowed []string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	for _, candidate := range allowed {
		if raw == candidate {
			return true
		}
	}
	return false
}
func appendQuery(raw, key, value string) string {
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

type GoogleHandler struct{ service *GoogleService }

func NewGoogleHandler(s *GoogleService) *GoogleHandler { return &GoogleHandler{service: s} }
func (h *GoogleHandler) Start(c *gin.Context) {
	var req GoogleStartRequest
	if c.ShouldBindJSON(&req) != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	out, e := h.service.Start(c.Request.Context(), req.ReturnURL)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
func (h *GoogleHandler) Callback(c *gin.Context) {
	location, e := h.service.Callback(c.Request.Context(), c.Query("state"), c.Query("code"))
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Redirect(http.StatusFound, location)
}
func (h *GoogleHandler) Exchange(c *gin.Context) {
	var req GoogleExchangeRequest
	if c.ShouldBindJSON(&req) != nil {
		models.WriteAppError(c, apperrors.ErrValidation)
		return
	}
	out, e := h.service.Exchange(c.Request.Context(), req.Code)
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	models.WriteOk(c, out)
}
