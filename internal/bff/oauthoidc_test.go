package bff

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOIDCTokenVerifier struct {
	claims oidcClaims
	err    error
	gotRaw string
}

func (f *fakeOIDCTokenVerifier) verifyOIDCToken(_ context.Context, raw string) (oidcClaims, error) {
	f.gotRaw = raw
	return f.claims, f.err
}

// tokenEndpoint stubs the provider's code-exchange endpoint: it asserts
// the client credentials are posted and returns a canned id_token.
func tokenEndpoint(t *testing.T, idToken string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "cid", r.Form.Get("client_id"))
		assert.Equal(t, "secret", r.Form.Get("client_secret"))
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "the-code", r.Form.Get("code"))
		assert.Equal(t, "http://app/cb", r.Form.Get("redirect_uri"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]string{"id_token": idToken})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOAuthOIDCVerifier(t *testing.T) {
	srv := tokenEndpoint(t, "the-id-token", http.StatusOK)
	tokens := &fakeOIDCTokenVerifier{claims: oidcClaims{
		issuer:        "https://login.microsoftonline.com/tid/v2.0",
		subject:       "ms-sub",
		email:         "u@ms.com",
		emailVerified: true,
		name:          "MS User",
		nonce:         "n1",
	}}
	v := NewOAuthOIDCVerifier("cid", "secret", "http://app/cb", srv.URL, tokens, false)
	require.NotNil(t, v)

	id, err := v.verify(t.Context(), "the-code", "n1")
	require.NoError(t, err)
	assert.Equal(t, "the-id-token", tokens.gotRaw)
	assert.Equal(t, "https://login.microsoftonline.com/tid/v2.0", id.provider)
	assert.Equal(t, "ms-sub", id.subject)
	assert.Equal(t, "u@ms.com", id.email)
	assert.True(t, id.emailVerified)
	assert.Equal(t, "MS User", id.name)
}

func TestOAuthOIDCVerifier_Nonce(t *testing.T) {
	srv := tokenEndpoint(t, "tok", http.StatusOK)

	// requireNonce=true with an empty nonce fails before the exchange.
	tokens := &fakeOIDCTokenVerifier{claims: oidcClaims{issuer: "iss", subject: "s", nonce: "n1"}}
	v := NewOAuthOIDCVerifier("cid", "secret", "http://app/cb", srv.URL, tokens, true)
	_, err := v.verify(t.Context(), "the-code", "")
	assert.ErrorIs(t, err, errOAuthCredential)
	assert.Empty(t, tokens.gotRaw)

	// A mismatched nonce is rejected.
	_, err = v.verify(t.Context(), "the-code", "other")
	assert.ErrorIs(t, err, errOAuthCredential)

	// Matching nonce passes.
	_, err = v.verify(t.Context(), "the-code", "n1")
	assert.NoError(t, err)

	// requireNonce=false still verifies a nonce when one is supplied.
	tokens2 := &fakeOIDCTokenVerifier{claims: oidcClaims{issuer: "iss", subject: "s", nonce: "n1"}}
	v2 := NewOAuthOIDCVerifier("cid", "secret", "http://app/cb", srv.URL, tokens2, false)
	_, err = v2.verify(t.Context(), "the-code", "wrong")
	assert.ErrorIs(t, err, errOAuthCredential)
	_, err = v2.verify(t.Context(), "the-code", "")
	assert.NoError(t, err)
}

func TestOAuthOIDCVerifier_ExchangeFails(t *testing.T) {
	srv := tokenEndpoint(t, "", http.StatusBadRequest)
	v := NewOAuthOIDCVerifier("cid", "secret", "http://app/cb", srv.URL, &fakeOIDCTokenVerifier{}, false)
	_, err := v.verify(t.Context(), "the-code", "")
	assert.ErrorIs(t, err, errOAuthCredential)
}

func TestNewOAuthOIDCVerifier_Unconfigured(t *testing.T) {
	tokens := &fakeOIDCTokenVerifier{}
	assert.Nil(t, NewOAuthOIDCVerifier("", "secret", "cb", "ep", tokens, false))
	assert.Nil(t, NewOAuthOIDCVerifier("id", "", "cb", "ep", tokens, false))
	assert.Nil(t, NewOAuthOIDCVerifier("id", "secret", "cb", "ep", nil, false))
	assert.NotNil(t, NewOAuthOIDCVerifier("id", "secret", "cb", "ep", tokens, false))
}

func TestMicrosoftIssuer(t *testing.T) {
	assert.Equal(t,
		"https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0",
		MicrosoftIssuer("consumers"))
	assert.Equal(t,
		"https://login.microsoftonline.com/11111111-2222-3333-4444-555555555555/v2.0",
		MicrosoftIssuer("11111111-2222-3333-4444-555555555555"))
	// Templated issuers can't be allowlisted — unsupported.
	assert.Empty(t, MicrosoftIssuer("common"))
	assert.Empty(t, MicrosoftIssuer("organizations"))
	assert.Empty(t, MicrosoftIssuer(""))
}

func TestTrustIssuer(t *testing.T) {
	issuers, audiences := TrustIssuer(
		[]string{"https://accounts.google.com"}, []string{"gid"},
		"https://www.facebook.com", "fb-app-id")
	assert.Equal(t, []string{"https://accounts.google.com", "https://www.facebook.com"}, issuers)
	assert.Equal(t, []string{"gid", "fb-app-id"}, audiences)

	// Already-present issuer is left alone (explicit config wins).
	issuers, audiences = TrustIssuer(issuers, audiences, "https://www.facebook.com", "other")
	assert.Len(t, issuers, 2)
	assert.Len(t, audiences, 2)

	// Empty inputs are no-ops.
	before := append([]string{}, issuers...)
	issuers, audiences = TrustIssuer(issuers, audiences, "", "x")
	issuers, audiences = TrustIssuer(issuers, audiences, "y", "")
	assert.Equal(t, before, issuers)
	assert.Len(t, audiences, 2)
}

// TestProviderKeysDistinct guards the registry keys from colliding with
// OIDC issuers or the "oidc"/"" dispatch sentinels.
func TestProviderKeysDistinct(t *testing.T) {
	for _, p := range []string{DiscordProvider, MicrosoftProvider, FacebookProvider} {
		parsed, err := url.Parse(p)
		require.NoError(t, err)
		assert.Empty(t, parsed.Scheme, "provider key %q must not be a URL", p)
		assert.NotContains(t, []string{"", "oidc"}, p)
	}
}
