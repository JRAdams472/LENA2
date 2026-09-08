package bff

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/platform/currentuser"
)

// fakeIdentityStore records UpsertUser/SetUserRole calls for assertions.
type fakeIdentityStore struct {
	user       identity.User
	inactive   bool
	upsertErr  error
	roleCalls  int32
	lastRole   string
	lastUserID int64
	roleErr    error
}

func (f *fakeIdentityStore) UpsertUser(_ context.Context, provider, subject, email, _ string) (identity.User, error) {
	if f.upsertErr != nil {
		return identity.User{}, f.upsertErr
	}
	u := f.user
	u.Provider = provider
	u.ExternalSubject = subject
	u.Email = email
	u.IsActive = !f.inactive
	return u, nil
}

func (f *fakeIdentityStore) SetUserRole(_ context.Context, userID int64, role string) error {
	atomic.AddInt32(&f.roleCalls, 1)
	f.lastUserID = userID
	f.lastRole = role
	return f.roleErr
}

// jwksIssuer is a test OIDC issuer serving discovery + a swappable JWKS.
type jwksIssuer struct {
	server *httptest.Server
	set    atomic.Value // jwk.Set
}

func newJWKSKey(t *testing.T, kid string) (*rsa.PrivateKey, jwk.Set) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pub, err := jwk.Import(priv.Public())
	if err != nil {
		t.Fatalf("import public key: %v", err)
	}
	_ = pub.Set(jwk.KeyIDKey, kid)
	_ = pub.Set(jwk.AlgorithmKey, jwa.RS256())
	set := jwk.NewSet()
	_ = set.AddKey(pub)
	return priv, set
}

func newJWKSIssuer(t *testing.T, initial jwk.Set) *jwksIssuer {
	t.Helper()
	iss := &jwksIssuer{}
	iss.set.Store(initial)
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   iss.server.URL,
			"jwks_uri": iss.server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(iss.set.Load())
	})
	iss.server = httptest.NewServer(mux)
	t.Cleanup(iss.server.Close)
	return iss
}

func signToken(t *testing.T, priv *rsa.PrivateKey, kid, issuer, audience, subject, email string, extraClaims map[string]any) string {
	t.Helper()
	key, err := jwk.Import(priv)
	if err != nil {
		t.Fatalf("import private key: %v", err)
	}
	_ = key.Set(jwk.KeyIDKey, kid)
	now := time.Now()
	b := jwt.NewBuilder().
		Issuer(issuer).
		Audience([]string{audience}).
		Subject(subject).
		IssuedAt(now).
		Expiration(now.Add(time.Hour)).
		Claim("email", email).
		Claim("name", "Test User")
	for k, v := range extraClaims {
		b = b.Claim(k, v)
	}
	tok, err := b.Build()
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), key))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return string(signed)
}

func TestAuthenticateValidToken(t *testing.T) {
	priv, set := newJWKSKey(t, "key-a")
	iss := newJWKSIssuer(t, set)
	store := &fakeIdentityStore{user: identity.User{UserID: 7, Role: identity.RoleMember}}
	a := NewAuthenticator(AuthConfig{
		Issuers:   []string{iss.server.URL},
		Audiences: []string{"lena-client"},
	}, store)

	raw := signToken(t, priv, "key-a", iss.server.URL, "lena-client", "sub-1", "user@example.com", nil)
	u, err := a.authenticate(context.Background(), raw)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if u.UserID != 7 || u.Email != "user@example.com" || u.IsAdmin {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestAuthenticateKeyRotation(t *testing.T) {
	privA, setA := newJWKSKey(t, "key-a")
	privB, setB := newJWKSKey(t, "key-b")
	iss := newJWKSIssuer(t, setA)
	store := &fakeIdentityStore{user: identity.User{UserID: 1, Role: identity.RoleMember}}
	a := NewAuthenticator(AuthConfig{
		Issuers:   []string{iss.server.URL},
		Audiences: []string{"lena-client"},
	}, store)

	// Warm the cache with key set A.
	rawA := signToken(t, privA, "key-a", iss.server.URL, "lena-client", "sub-1", "u@example.com", nil)
	if _, err := a.authenticate(context.Background(), rawA); err != nil {
		t.Fatalf("warm-up authenticate: %v", err)
	}

	// Issuer rotates to key set B; a token signed with B fails the first
	// verify against the cached set A, then succeeds after the forced
	// cache-bust re-fetch. Age the cached entry past the minimum refresh
	// interval so the forced refresh is allowed to hit the issuer.
	a.mu.Lock()
	a.jwks[iss.server.URL].fetchedAt = time.Now().Add(-jwksMinRefreshInterval)
	a.mu.Unlock()
	iss.set.Store(setB)
	rawB := signToken(t, privB, "key-b", iss.server.URL, "lena-client", "sub-1", "u@example.com", nil)
	u, err := a.authenticate(context.Background(), rawB)
	if err != nil {
		t.Fatalf("authenticate after rotation: %v", err)
	}
	if u.UserID != 1 {
		t.Fatalf("unexpected user id %d", u.UserID)
	}
}

func TestAuthenticateRejects(t *testing.T) {
	priv, set := newJWKSKey(t, "key-a")
	iss := newJWKSIssuer(t, set)
	store := &fakeIdentityStore{user: identity.User{UserID: 1}}
	a := NewAuthenticator(AuthConfig{
		Issuers:   []string{iss.server.URL},
		Audiences: []string{"lena-client"},
	}, store)

	t.Run("malformed token", func(t *testing.T) {
		if _, err := a.authenticate(context.Background(), "not-a-jwt"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("disallowed issuer", func(t *testing.T) {
		raw := signToken(t, priv, "key-a", "https://evil.example.com", "lena-client", "s", "e@x.com", nil)
		if _, err := a.authenticate(context.Background(), raw); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("wrong audience", func(t *testing.T) {
		raw := signToken(t, priv, "key-a", iss.server.URL, "other-client", "s", "e@x.com", nil)
		if _, err := a.authenticate(context.Background(), raw); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("upsert failure", func(t *testing.T) {
		bad := &fakeIdentityStore{upsertErr: errors.New("db down")}
		a2 := NewAuthenticator(AuthConfig{
			Issuers:   []string{iss.server.URL},
			Audiences: []string{"lena-client"},
		}, bad)
		raw := signToken(t, priv, "key-a", iss.server.URL, "lena-client", "s", "e@x.com", nil)
		if _, err := a2.authenticate(context.Background(), raw); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("banned user is rejected", func(t *testing.T) {
		banned := &fakeIdentityStore{user: identity.User{UserID: 5, Role: identity.RoleMember}, inactive: true}
		a2 := NewAuthenticator(AuthConfig{
			Issuers:   []string{iss.server.URL},
			Audiences: []string{"lena-client"},
		}, banned)
		raw := signToken(t, priv, "key-a", iss.server.URL, "lena-client", "s", "e@x.com", nil)
		_, err := a2.authenticate(context.Background(), raw)
		require.Error(t, err)
		assert.ErrorContains(t, err, "disabled")
	})
	t.Run("banned admin-list email is not promoted", func(t *testing.T) {
		banned := &fakeIdentityStore{user: identity.User{UserID: 6, Role: identity.RoleMember}, inactive: true}
		a2 := NewAuthenticator(AuthConfig{
			Issuers:     []string{iss.server.URL},
			Audiences:   []string{"lena-client"},
			AdminEmails: []string{"e@x.com"},
		}, banned)
		raw := signToken(t, priv, "key-a", iss.server.URL, "lena-client", "s", "e@x.com",
			map[string]any{"email_verified": true})
		_, err := a2.authenticate(context.Background(), raw)
		require.Error(t, err)
		assert.Zero(t, atomic.LoadInt32(&banned.roleCalls))
	})
}

func TestAuthenticateAdminPromotion(t *testing.T) {
	priv, set := newJWKSKey(t, "key-a")
	iss := newJWKSIssuer(t, set)
	store := &fakeIdentityStore{user: identity.User{UserID: 9, Role: identity.RoleMember}}
	a := NewAuthenticator(AuthConfig{
		Issuers:     []string{iss.server.URL},
		Audiences:   []string{"lena-client"},
		AdminEmails: []string{"admin@example.com"},
	}, store)

	raw := signToken(t, priv, "key-a", iss.server.URL, "lena-client", "sub-9", "admin@example.com",
		map[string]any{"email_verified": true})
	u, err := a.authenticate(context.Background(), raw)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if !u.IsAdmin {
		t.Fatal("expected promoted user to be admin")
	}
	if atomic.LoadInt32(&store.roleCalls) != 1 || store.lastUserID != 9 || store.lastRole != identity.RoleAdmin {
		t.Fatalf("expected SetUserRole(9, admin), got calls=%d uid=%d role=%s", store.roleCalls, store.lastUserID, store.lastRole)
	}
}

// An allowlisted email must not promote when the provider did not verify
// it, or when the claim is absent entirely.
func TestAuthenticateAdminPromotionRequiresVerifiedEmail(t *testing.T) {
	priv, set := newJWKSKey(t, "key-a")
	iss := newJWKSIssuer(t, set)

	for _, tc := range []struct {
		name  string
		extra map[string]any
	}{
		{name: "claim absent", extra: nil},
		{name: "claim false", extra: map[string]any{"email_verified": false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeIdentityStore{user: identity.User{UserID: 9, Role: identity.RoleMember}}
			a := NewAuthenticator(AuthConfig{
				Issuers:     []string{iss.server.URL},
				Audiences:   []string{"lena-client"},
				AdminEmails: []string{"admin@example.com"},
			}, store)

			raw := signToken(t, priv, "key-a", iss.server.URL, "lena-client", "sub-9", "admin@example.com", tc.extra)
			u, err := a.authenticate(context.Background(), raw)
			if err != nil {
				t.Fatalf("authenticate: %v", err)
			}
			if u.IsAdmin {
				t.Fatal("unverified email must not be promoted to admin")
			}
			if atomic.LoadInt32(&store.roleCalls) != 0 {
				t.Fatalf("expected no SetUserRole call, got %d", store.roleCalls)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	priv, set := newJWKSKey(t, "key-a")
	iss := newJWKSIssuer(t, set)
	store := &fakeIdentityStore{user: identity.User{UserID: 3}}
	a := NewAuthenticator(AuthConfig{
		Issuers:   []string{iss.server.URL},
		Audiences: []string{"lena-client"},
	}, store)

	e := echo.New()
	e.Use(a.Middleware())
	e.POST("/graphql", func(c echo.Context) error {
		u, ok := currentuser.FromContext(c.Request().Context())
		if !ok || u.UserID != 3 {
			return c.NoContent(http.StatusInternalServerError)
		}
		return c.NoContent(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: got %d, want 401", rec.Code)
	}

	raw := signToken(t, priv, "key-a", iss.server.URL, "lena-client", "sub-3", "u@example.com", nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+raw)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid token: got %d, want 200", rec.Code)
	}
}

func TestExtractBearer(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		want    string
		wantErr bool
	}{
		{name: "valid", header: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "missing header", header: "", wantErr: true},
		{name: "wrong scheme", header: "Basic abc", wantErr: true},
		{name: "empty token", header: "Bearer ", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/graphql", nil)
			if err != nil {
				t.Fatalf("unexpected error building request: %v", err)
			}
			if tc.header != "" {
				req.Header.Set(echo.HeaderAuthorization, tc.header)
			}

			got, err := extractBearer(req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestContainsFold(t *testing.T) {
	list := []string{"Admin@Example.com"}
	if !containsFold(list, "admin@example.com") {
		t.Fatal("expected case-insensitive match")
	}
	if containsFold(list, "other@example.com") {
		t.Fatal("expected no match")
	}
}

func TestContainsAny(t *testing.T) {
	allowed := []string{"client-a", "client-b"}
	if !containsAny(allowed, []string{"client-c", "client-b"}) {
		t.Fatal("expected match on client-b")
	}
	if containsAny(allowed, []string{"client-c", "client-d"}) {
		t.Fatal("expected no match")
	}
}
