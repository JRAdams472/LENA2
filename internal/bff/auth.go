// Package bff hosts the GraphQL BFF: authentication, orchestration across
// domain modules, and the schema/resolvers exposed to Flutter and Next.js.
package bff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

// AuthConfig configures which OIDC issuers and audiences are trusted.
// Audiences are paired with issuers by position, so a token's audience is
// only accepted when it matches the audience entry at the same index as its
// issuer. AdminEmails entries should be qualified as "issuer:email" so the
// same email cannot be promoted from an unrelated trusted issuer.
type AuthConfig struct {
	Issuers   []string
	Audiences []string
	// AdminEmails is a bootstrap allowlist: a user whose *provider-verified*
	// email is listed is promoted to the persisted 'admin' role on their
	// next request. Tokens without email_verified=true are never promoted.
	// Entries may be qualified as "issuer:email" (use the last colon as the
	// separator). A bare email is accepted only when exactly one issuer is
	// configured and is implicitly scoped to that issuer.
	AdminEmails []string
}

// identityStore is the subset of identity.Service the authenticator needs;
// declared as an interface so tests can substitute a fake.
type identityStore interface {
	UpsertUser(ctx context.Context, provider, subject, email, displayName string) (identity.User, error)
	SetUserRole(ctx context.Context, userID int64, role string) error
}

// Authenticator validates OIDC ID tokens and resolves the current user.
type Authenticator struct {
	identity            identityStore
	audiencesByIssuer   map[string][]string
	adminEmailsByIssuer map[string][]string
	httpc               *http.Client

	// jwks is guarded by an RW mutex: cache hits never block on a network
	// fetch, and per-issuer singleflight (inflight) means a slow provider
	// only delays tokens from that issuer — never the whole auth path.
	mu       sync.RWMutex
	jwks     map[string]*cachedKeySet
	inflight map[string]*jwksFetch

	// users caches the resolved identity row per issuer|subject so a burst
	// of requests from one token does not UpsertUser on every call.
	userMu sync.Mutex
	users  map[string]resolvedUser
}

type cachedKeySet struct {
	set       jwk.Set
	fetchedAt time.Time
	expiresAt time.Time
}

// jwksFetch coalesces concurrent refreshes for one issuer; waiters close
// over done and read set/err once it closes.
type jwksFetch struct {
	done chan struct{}
	set  jwk.Set
	err  error
}

// resolvedUser is a cached identity resolution result.
type resolvedUser struct {
	user      currentuser.User
	expiresAt time.Time
}

const (
	jwksCacheTTL = time.Hour
	// jwksMinRefreshInterval bounds how often a forced refresh can hit the
	// issuer: bursts of tokens signed by unknown keys must not amplify into
	// JWKS fetch storms against the provider.
	jwksMinRefreshInterval = 30 * time.Second
	// jwksFetchTimeout bounds the detached discovery+JWKS fetch. It is
	// independent of the request context so one slow issuer cannot stall
	// — or be cancelled by — unrelated requests.
	jwksFetchTimeout = 10 * time.Second
	// maxOIDCDocBytes bounds the OIDC discovery document and JWKS response
	// bodies so a hostile or misbehaving endpoint cannot exhaust memory.
	maxOIDCDocBytes = 1 << 20
	// userCacheTTL bounds how long a resolved user row is reused. A ban or
	// role change can lag by at most this much; the TTL is further capped
	// by the token's own expiry.
	userCacheTTL = 2 * time.Minute
	// userCacheMax bounds the resolved-user map; when full, expired
	// entries are swept before inserting.
	userCacheMax = 4096
)

// Sentinel auth errors let the middleware distinguish "your token is bad",
// "the identity store is unreachable", and "the issuer's JWKS endpoint is
// unreachable", mapping each to the appropriate HTTP status code.
var (
	errTokenInvalid  = errors.New("invalid token")
	errKeyDiscovery  = errors.New("key discovery failed")
	errIdentityStore = errors.New("identity store unavailable")
	errAccountBanned = errors.New("account is disabled")
)

// NewAuthenticator creates an Authenticator backed by the given identity service.
func NewAuthenticator(cfg AuthConfig, identitySvc identityStore) (*Authenticator, error) {
	audiences, err := zipIssuersAndAudiences(cfg.Issuers, cfg.Audiences)
	if err != nil {
		return nil, fmt.Errorf("auth config: %w", err)
	}
	adminEmails, err := groupAdminEmailsByIssuer(cfg.Issuers, cfg.AdminEmails)
	if err != nil {
		return nil, fmt.Errorf("auth config: %w", err)
	}
	return &Authenticator{
		identity:            identitySvc,
		audiencesByIssuer:   audiences,
		adminEmailsByIssuer: adminEmails,
		httpc: &http.Client{
			Timeout: 10 * time.Second,
			// OIDC discovery and JWKS endpoints must be fetched directly:
			// following a redirect could silently re-target key material
			// to an attacker-controlled host.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		jwks:     make(map[string]*cachedKeySet),
		inflight: make(map[string]*jwksFetch),
		users:    make(map[string]resolvedUser),
	}, nil
}

// zipIssuersAndAudiences pairs the ith issuer with the ith audience. This
// prevents a token from issuer A being accepted solely because it carries
// an audience belonging to issuer B.
func zipIssuersAndAudiences(issuers, audiences []string) (map[string][]string, error) {
	if len(issuers) == 0 {
		return nil, errors.New("at least one issuer is required")
	}
	if len(audiences) == 0 {
		return nil, errors.New("at least one audience is required")
	}
	if len(issuers) != len(audiences) {
		return nil, fmt.Errorf("issuer count (%d) does not match audience count (%d); each issuer must have exactly one audience", len(issuers), len(audiences))
	}
	m := make(map[string][]string, len(issuers))
	for i, iss := range issuers {
		m[iss] = []string{audiences[i]}
	}
	return m, nil
}

// groupAdminEmailsByIssuer parses "issuer:email" entries into per-issuer
// admin allowlists. A bare email is only allowed when a single issuer is
// configured; otherwise the issuer prefix is required for safety.
func groupAdminEmailsByIssuer(issuers, adminEmails []string) (map[string][]string, error) {
	byIssuer := make(map[string][]string)
	if len(adminEmails) == 0 {
		return byIssuer, nil
	}
	singleIssuer := len(issuers) == 1
	for _, raw := range adminEmails {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		issuer, email := splitProviderScoped(raw)
		if issuer == "" {
			if !singleIssuer {
				return nil, fmt.Errorf("admin email %q is missing an issuer qualifier; required when multiple issuers are configured", raw)
			}
			issuer = issuers[0]
		}
		if !slices.Contains(issuers, issuer) {
			return nil, fmt.Errorf("admin email %q references unknown issuer %q", raw, issuer)
		}
		byIssuer[issuer] = append(byIssuer[issuer], email)
	}
	return byIssuer, nil
}

// splitProviderScoped splits "provider:value" at the last colon, which is
// safe because email addresses cannot contain colons. It returns an empty
// provider when no qualifier is present.
func splitProviderScoped(s string) (provider, value string) {
	if i := strings.LastIndex(s, ":"); i > 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return "", strings.TrimSpace(s)
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
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}

			ctx := c.Request().Context()
			user, err := a.authenticate(ctx, raw)
			if err != nil {
				a.logAuthError(c, err)
				if errors.Is(err, errKeyDiscovery) || errors.Is(err, errIdentityStore) {
					c.Response().Header().Set("Retry-After", "5")
					return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
				}
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}

			c.SetRequest(c.Request().WithContext(currentuser.WithUser(ctx, user)))
			return next(c)
		}
	}
}

// logAuthError records the cause of an authentication failure for ops and
// security review, redacting the raw token itself.
func (a *Authenticator) logAuthError(c echo.Context, err error) {
	requestID := c.Response().Header().Get(echo.HeaderXRequestID)
	issuer := c.Request().Header.Get("X-Issuer")
	subject := c.Request().Header.Get("X-Subject")
	switch {
	case errors.Is(err, errKeyDiscovery), errors.Is(err, errIdentityStore):
		slog.Default().Error("auth backend failure",
			"request_id", requestID,
			"issuer", issuer,
			"subject_hash", hashSubject(subject),
			"reason", err.Error(),
		)
	default:
		slog.Default().Warn("auth failure",
			"request_id", requestID,
			"issuer", issuer,
			"subject_hash", hashSubject(subject),
			"reason", err.Error(),
		)
	}
}

// hashSubject provides a short, stable identifier for logging without
// exposing the raw external subject.
func hashSubject(subject string) string {
	if subject == "" {
		return ""
	}
	const limit = 16
	if len(subject) <= limit {
		return subject
	}
	return subject[:limit]
}

func (a *Authenticator) authenticate(ctx context.Context, raw string) (currentuser.User, error) {
	unverified, err := jwt.ParseInsecure([]byte(raw))
	if err != nil {
		return currentuser.User{}, fmt.Errorf("%w: parse token: %w", errTokenInvalid, err)
	}

	issuer, ok := unverified.Issuer()
	if !ok {
		return currentuser.User{}, fmt.Errorf("%w: token has no issuer", errTokenInvalid)
	}
	if _, ok := a.audiencesByIssuer[issuer]; !ok {
		return currentuser.User{}, fmt.Errorf("%w: issuer %q is not allowed", errTokenInvalid, issuer)
	}

	keySet, err := a.keySetForIssuer(ctx, issuer, false)
	if err != nil {
		return currentuser.User{}, fmt.Errorf("%w: load jwks for issuer %q: %w", errKeyDiscovery, issuer, err)
	}

	token, err := jwt.Parse([]byte(raw), jwt.WithKeySet(keySet), jwt.WithValidate(true))
	if err != nil {
		// The issuer may have rotated signing keys inside our cache window.
		// Only a token whose kid is absent from the cached set can be fixed
		// by re-fetching keys; any other failure (expiry, bad claims) is not
		// worth a refresh. The forced refresh is rate-limited inside
		// keySetForIssuer.
		if kid := signingKeyID(raw); kid == "" || !a.cachedSetHasKey(issuer, kid) {
			if refreshed, refErr := a.keySetForIssuer(ctx, issuer, true); refErr == nil {
				token, err = jwt.Parse([]byte(raw), jwt.WithKeySet(refreshed), jwt.WithValidate(true))
			}
		}
		if err != nil {
			return currentuser.User{}, fmt.Errorf("%w: verify token: %w", errTokenInvalid, err)
		}
	}

	audience, ok := token.Audience()
	if !ok {
		return currentuser.User{}, fmt.Errorf("%w: token has no audience claim", errTokenInvalid)
	}
	if !containsAny(a.audiencesByIssuer[issuer], audience) {
		return currentuser.User{}, fmt.Errorf("%w: audience %v is not allowed for issuer %q", errTokenInvalid, audience, issuer)
	}

	subject, ok := token.Subject()
	if !ok || subject == "" {
		return currentuser.User{}, fmt.Errorf("%w: token has no subject", errTokenInvalid)
	}

	var email, name string
	if err := token.Get("email", &email); err != nil {
		return currentuser.User{}, fmt.Errorf("%w: token email claim: %w", errTokenInvalid, err)
	}
	if err := token.Get("name", &name); err != nil {
		return currentuser.User{}, fmt.Errorf("%w: token name claim: %w", errTokenInvalid, err)
	}

	// A missing or non-boolean claim simply means "not verified".
	var emailVerified bool
	_ = token.Get("email_verified", &emailVerified)

	if cu, ok := a.cachedUser(issuer, subject); ok {
		return cu, nil
	}

	u, err := a.identity.UpsertUser(ctx, issuer, subject, email, name)
	if err != nil {
		return currentuser.User{}, fmt.Errorf("%w: upsert user: %w", errIdentityStore, err)
	}

	// Banned users are rejected on every request: the JWT stays
	// cryptographically valid until expiry, but the API refuses to serve
	// the account while is_active is false.
	if !u.IsActive {
		return currentuser.User{}, fmt.Errorf("%w: account is disabled", errAccountBanned)
	}

	// Bootstrap admin access: users whose provider-verified email is
	// configured in LENA_ADMIN_EMAILS are promoted to the persisted admin
	// role here so the role survives across logins. An unverified or absent
	// email claim must never trigger promotion. The admin email must be
	// scoped to the same issuer the token came from.
	if adminList := a.adminEmailsByIssuer[issuer]; u.Role != identity.RoleAdmin && emailVerified && containsFold(adminList, u.Email) {
		if err := a.identity.SetUserRole(ctx, u.UserID, identity.RoleAdmin); err != nil {
			return currentuser.User{}, fmt.Errorf("%w: promote admin user: %w", errIdentityStore, err)
		}
		u.Role = identity.RoleAdmin
		slog.Default().Info("audit",
			"action", "promote_admin",
			"actor", u.Email,
			"target_id", u.UserID,
			"provider", issuer,
		)
	}

	cu := currentuser.User{
		UserID:          u.UserID,
		Provider:        u.Provider,
		ExternalSubject: u.ExternalSubject,
		Email:           u.Email,
		DisplayName:     u.DisplayName,
		IsAdmin:         u.IsAdmin(),
	}
	// Cap the cache entry by the token's own expiry so a cached resolution
	// never outlives the credential it was minted for.
	ttl := userCacheTTL
	if exp, ok := token.Expiration(); ok {
		if rem := time.Until(exp); rem < ttl {
			ttl = rem
		}
	}
	a.cacheUser(issuer, subject, cu, ttl)
	return cu, nil
}

// cachedUser returns the previously resolved identity for issuer|subject
// when it is still fresh. A role change or ban can lag by at most
// userCacheTTL — the tradeoff accepted to keep a request burst from
// hammering UpsertUser.
func (a *Authenticator) cachedUser(issuer, subject string) (currentuser.User, bool) {
	a.userMu.Lock()
	defer a.userMu.Unlock()
	e, ok := a.users[issuer+"\x00"+subject]
	if !ok || time.Now().After(e.expiresAt) {
		return currentuser.User{}, false
	}
	return e.user, true
}

// cacheUser stores the resolved user, sweeping expired entries when the
// map grows past userCacheMax.
func (a *Authenticator) cacheUser(issuer, subject string, u currentuser.User, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	a.userMu.Lock()
	defer a.userMu.Unlock()
	if len(a.users) >= userCacheMax {
		now := time.Now()
		for k, e := range a.users {
			if now.After(e.expiresAt) {
				delete(a.users, k)
			}
		}
	}
	a.users[issuer+"\x00"+subject] = resolvedUser{user: u, expiresAt: time.Now().Add(ttl)}
}

// keySetForIssuer returns a cached JWKS for the issuer, refreshing it when
// the cache is empty, stale, or forceRefresh is set (used after a
// signature-verification failure to handle key rotation).
//
// Network fetches are never made under the lock: concurrent refreshes for
// one issuer coalesce onto a single in-flight fetch, callers holding a
// usable (even stale) cached set return it immediately while the refresh
// proceeds, and the fetch runs on a detached context so one request's
// cancellation cannot kill the shared refresh for the issuer.
func (a *Authenticator) keySetForIssuer(ctx context.Context, issuer string, forceRefresh bool) (jwk.Set, error) {
	now := time.Now()

	a.mu.RLock()
	entry, cached := a.jwks[issuer]
	a.mu.RUnlock()

	if cached {
		if !forceRefresh && now.Before(entry.expiresAt) {
			return entry.set, nil
		}
		if forceRefresh && now.Sub(entry.fetchedAt) < jwksMinRefreshInterval {
			return entry.set, nil
		}
	}

	a.mu.Lock()
	if f, running := a.inflight[issuer]; running {
		a.mu.Unlock()
		// Stale-while-revalidate: a non-forced caller with any cached set
		// uses it rather than queuing behind the in-flight fetch. A forced
		// refresh (key rotation) must wait for the fresh set — the stale
		// one provably lacks the token's kid.
		if cached && !forceRefresh {
			return entry.set, nil
		}
		select {
		case <-f.done:
			return f.set, f.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f := &jwksFetch{done: make(chan struct{})}
	a.inflight[issuer] = f
	a.mu.Unlock()

	complete := func(set jwk.Set, err error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if err == nil {
			a.jwks[issuer] = &cachedKeySet{set: set, fetchedAt: time.Now(), expiresAt: time.Now().Add(jwksCacheTTL)}
		}
		f.set, f.err = set, err
		close(f.done)
		delete(a.inflight, issuer)
	}

	// Detached from the request: one caller's cancellation must not kill
	// the shared refresh for everyone else authenticating to this issuer.
	if cached && !forceRefresh {
		// A stale-but-usable set is served immediately while the refresh
		// proceeds in the background.
		go func() {
			fetchCtx, cancel := context.WithTimeout(context.Background(), jwksFetchTimeout)
			defer cancel()
			set, err := a.fetchKeySet(fetchCtx, issuer)
			complete(set, err)
		}()
		return entry.set, nil
	}
	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), jwksFetchTimeout)
	set, err := a.fetchKeySet(fetchCtx, issuer)
	cancel()
	complete(set, err)
	if err != nil && cached {
		// A failed forced refresh still falls back to the cached set; the
		// signature check against it is what actually decides validity.
		return entry.set, nil
	}
	return set, err
}

// fetchKeySet performs the OIDC discovery + JWKS fetch for an issuer on
// the given context.
func (a *Authenticator) fetchKeySet(ctx context.Context, issuer string) (jwk.Set, error) {
	jwksURI, err := a.discoverJWKSURI(ctx, issuer)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch jwks: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOIDCDocBytes))
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: read body: %w", err)
	}
	set, err := jwk.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: parse body: %w", err)
	}
	return set, nil
}

// cachedSetHasKey reports whether the issuer's cached JWKS already
// contains the given key id.
func (a *Authenticator) cachedSetHasKey(issuer, kid string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	entry, ok := a.jwks[issuer]
	if !ok {
		return false
	}
	_, found := entry.set.LookupKeyID(kid)
	return found
}

// signingKeyID extracts the JWS protected header's kid without verifying
// the signature; it is used only to decide whether a JWKS refresh could
// possibly help.
func signingKeyID(raw string) string {
	msg, err := jws.Parse([]byte(raw))
	if err != nil || len(msg.Signatures()) == 0 {
		return ""
	}
	kid, _ := msg.Signatures()[0].ProtectedHeaders().KeyID()
	return kid
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discovery request to %s returned status %d", discoveryURL, resp.StatusCode)
	}

	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOIDCDocBytes)).Decode(&doc); err != nil {
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

// containsFold reports whether value matches a list entry
// case-insensitively, as is correct for email comparisons.
func containsFold(list []string, value string) bool {
	return slices.ContainsFunc(list, func(v string) bool {
		return strings.EqualFold(v, value)
	})
}

func containsAny(allowed, values []string) bool {
	return slices.ContainsFunc(values, func(v string) bool {
		return slices.Contains(allowed, v)
	})
}
