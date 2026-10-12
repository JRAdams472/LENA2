package bff

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestKey(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	require.NoError(t, os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))
	return path
}

func TestLoadApplePrivateKey(t *testing.T) {
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	path := writeTestKey(t, ec)

	got, err := LoadApplePrivateKey(path)
	require.NoError(t, err)
	assert.True(t, ec.Equal(got))

	// Missing file and non-PEM content are clean errors, not panics.
	_, err = LoadApplePrivateKey(filepath.Join(t.TempDir(), "nope.p8"))
	assert.Error(t, err)
	bad := filepath.Join(t.TempDir(), "bad.p8")
	require.NoError(t, os.WriteFile(bad, []byte("not pem"), 0o600))
	_, err = LoadApplePrivateKey(bad)
	assert.Error(t, err)

	// RSA and non-P256 keys are rejected — Apple only accepts ES256.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	_, err = LoadApplePrivateKey(writeTestKey(t, rsaKey))
	assert.Error(t, err)
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	_, err = LoadApplePrivateKey(writeTestKey(t, p384))
	assert.Error(t, err)
}

// verifyAppleSecret parses the minted client_secret and checks the ES256
// signature against the private key's public half — the exact check
// Apple runs on its token endpoint.
func verifyAppleSecret(t *testing.T, key *ecdsa.PrivateKey, raw string) jwt.Token {
	t.Helper()
	pub, err := jwk.Import(&key.PublicKey)
	require.NoError(t, err)
	msg, err := jws.Parse([]byte(raw))
	require.NoError(t, err)
	sigs := msg.Signatures()
	require.Len(t, sigs, 1)
	kid, ok := sigs[0].ProtectedHeaders().KeyID()
	require.True(t, ok)
	assert.Equal(t, "KEY123", kid)

	tok, err := jwt.Parse([]byte(raw), jwt.WithKey(jwa.ES256(), pub))
	require.NoError(t, err)
	return tok
}

func TestAppleClientSecret(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	raw, err := appleClientSecret("TEAM1", "KEY123", "com.example.lena.web", key, time.Now())
	require.NoError(t, err)

	tok := verifyAppleSecret(t, key, raw)
	iss, _ := tok.Issuer()
	sub, _ := tok.Subject()
	aud, _ := tok.Audience()
	assert.Equal(t, "TEAM1", iss)
	assert.Equal(t, "com.example.lena.web", sub)
	assert.Equal(t, []string{appleIssuer}, aud)
	exp, _ := tok.Expiration()
	iat, _ := tok.IssuedAt()
	assert.Equal(t, appleSecretTTL, exp.Sub(iat))
}

// appleTokenEndpointStub stubs Apple's token endpoint: it verifies the
// client_secret JWT signature and claims, then returns a canned
// id_token.
func appleTokenEndpointStub(t *testing.T, key *ecdsa.PrivateKey, idToken string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "com.example.lena.web", r.Form.Get("client_id"))
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "the-code", r.Form.Get("code"))
		verifyAppleSecret(t, key, r.Form.Get("client_secret"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": idToken})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAppleVerifier_Exchange(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	srv := appleTokenEndpointStub(t, key, "apple-id-token")
	tokens := &fakeOIDCTokenVerifier{claims: oidcClaims{
		issuer:        appleIssuer,
		subject:       "apple-sub",
		email:         "u@privaterelay.appleid.com",
		emailVerified: true,
		nonce:         "n1",
	}}

	v := NewAppleVerifier("com.example.lena.web", "TEAM1", "KEY123",
		"http://app/cb", key, tokens)
	require.NotNil(t, v)
	v.tokenEndpoint = srv.URL

	// Apple requires the authorize-request nonce — empty fails before
	// any outbound call.
	_, err = v.verify(t.Context(), "the-code", "", "v1", "")
	assert.ErrorIs(t, err, errOAuthCredential)
	assert.Empty(t, tokens.gotRaw)

	id, err := v.verify(t.Context(), "the-code", "n1", "v1", "")
	require.NoError(t, err)
	assert.Equal(t, "apple-id-token", tokens.gotRaw)
	assert.Equal(t, appleIssuer, id.provider)
	assert.Equal(t, "apple-sub", id.subject)
	assert.True(t, id.emailVerified)
}

func TestNewAppleVerifier_Unconfigured(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tokens := &fakeOIDCTokenVerifier{}

	assert.Nil(t, NewAppleVerifier("", "TEAM1", "KEY123", "cb", key, tokens))
	assert.Nil(t, NewAppleVerifier("id", "", "KEY123", "cb", key, tokens))
	assert.Nil(t, NewAppleVerifier("id", "TEAM1", "", "cb", key, tokens))
	assert.Nil(t, NewAppleVerifier("id", "TEAM1", "KEY123", "cb", nil, tokens))
	assert.Nil(t, NewAppleVerifier("id", "TEAM1", "KEY123", "cb", key, nil))
	assert.NotNil(t, NewAppleVerifier("id", "TEAM1", "KEY123", "cb", key, tokens))
}
