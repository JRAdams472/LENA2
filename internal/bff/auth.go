// Package bff hosts the GraphQL BFF: authentication, orchestration across
// domain modules, and the schema/resolvers exposed to Flutter and Next.js.
package bff

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

// AuthConfig configures which OIDC issuers and audiences are trusted.
// Adding a provider later only requires appending to these lists.
type AuthConfig struct {
	Issuers   []string
	Audiences []string
}

// Authenticator validates OIDC ID tokens and resolves the current user.
type Authenticator struct {
	cfg      AuthConfig
	identity *identity.Service
	httpc    *http.Client

	mu   sync.Mutex
	jwks map[string]*cachedKeySet
}

type cachedKeySet struct {
	set       jwk.Set
	expiresAt time.Time
}

const jwksCacheTTL = time.Hour

// NewAuthenticator creates an Authenticator backed by the given identity service.
func NewAuthenticator(cfg AuthConfig, identitySvc *identity.Service) *Authenticator {
	return &Authenticator{
		cfg:      cfg,
		identity: identitySvc,
		httpc:    &http.Client{Timeout: 10 * time.Second},
		jwks:     make(map[string]*cachedKeySet),
	}
}

// Middleware validates the bearer token on every request, upserts the
// corresponding identity.users row, and stores the CurrentUser in context.
// Requests without a valid token are rejected with 401 before reaching any
// handler.
func (a *Authenticator) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			raw, err := extractBearer(c.Request())
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}

			ctx := c.Request().Context()
			user, err := a.authenticate(ctx, raw)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			c.SetRequest(c.Request().WithContext(currentuser.WithUser(ctx, user)))
			return next(c)
		}
	}
}

func (a *Authenticator) authenticate(ctx context.Context, raw string) (currentuser.User, error) {
	unverified, err := jwt.ParseInsecure([]byte(raw))
	if err != nil {
		return currentuser.User{}, fmt.Errorf("parse token: %w", err)
	}

	issuer, ok := unverified.Issuer()
	if !ok || !contains(a.cfg.Issuers, issuer) {
		return currentuser.User{}, fmt.Errorf("issuer %q is not allowed", issuer)
	}

	keySet, err := a.keySetForIssuer(ctx, issuer)
	if err != nil {
		return currentuser.User{}, fmt.Errorf("load jwks for issuer %q: %w", issuer, err)
	}

	token, err := jwt.Parse([]byte(raw), jwt.WithKeySet(keySet), jwt.WithValidate(true))
	if err != nil {
		return currentuser.User{}, fmt.Errorf("verify token: %w", err)
	}

	audience, _ := token.Audience()
	if !containsAny(a.cfg.Audiences, audience) {
		return currentuser.User{}, fmt.Errorf("audience %v is not allowed", audience)
	}

	subject, ok := token.Subject()
	if !ok || subject == "" {
		return currentuser.User{}, fmt.Errorf("token has no subject")
	}

	var email, name string
	_ = token.Get("email", &email)
	_ = token.Get("name", &name)

	u, err := a.identity.UpsertUser(ctx, issuer, subject, email, name)
	if err != nil {
		return currentuser.User{}, fmt.Errorf("upsert user: %w", err)
	}

	return currentuser.User{
		UserID:          u.UserID,
		Provider:        u.Provider,
		ExternalSubject: u.ExternalSubject,
		Email:           u.Email,
		DisplayName:     u.DisplayName,
	}, nil
}

// keySetForIssuer returns a cached JWKS for the issuer, discovering and
// fetching it if the cache is empty or stale.
func (a *Authenticator) keySetForIssuer(ctx context.Context, issuer string) (jwk.Set, error) {
	a.mu.Lock()
	entry, ok := a.jwks[issuer]
	a.mu.Unlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.set, nil
	}

	jwksURI, err := a.discoverJWKSURI(ctx, issuer)
	if err != nil {
		return nil, err
	}

	set, err := jwk.Fetch(ctx, jwksURI)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}

	a.mu.Lock()
	a.jwks[issuer] = &cachedKeySet{set: set, expiresAt: time.Now().Add(jwksCacheTTL)}
	a.mu.Unlock()

	return set, nil
}

// discoverJWKSURI resolves the issuer's JWKS endpoint via OIDC discovery
// (RFC/OIDC well-known configuration), so adding a new OIDC provider only
// requires adding its issuer/audience to configuration.
func (a *Authenticator) discoverJWKSURI(ctx context.Context, issuer string) (string, error) {
	discoveryURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := a.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discovery request to %s returned status %d", discoveryURL, resp.StatusCode)
	}

	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("decode discovery document: %w", err)
	}
	if doc.JWKSURI == "" {
		return "", fmt.Errorf("discovery document for %s has no jwks_uri", issuer)
	}

	return doc.JWKSURI, nil
}

func extractBearer(r *http.Request) (string, error) {
	header := r.Header.Get(echo.HeaderAuthorization)
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("authorization header missing bearer token")
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", fmt.Errorf("authorization header missing bearer token")
	}
	return token, nil
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func containsAny(allowed, values []string) bool {
	for _, v := range values {
		if contains(allowed, v) {
			return true
		}
	}
	return false
}
