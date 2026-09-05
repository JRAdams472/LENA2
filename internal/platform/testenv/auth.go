package testenv

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// TestAudience is the audience claim expected by test Authenticators.
const TestAudience = "lena-test-client"

// TestIssuer is a throwaway OIDC issuer backed by an httptest server. It
// serves a discovery document and a JWKS containing a single RSA key, and
// can mint signed ID tokens that an Authenticator configured with this
// issuer will accept.
type TestIssuer struct {
	URL      string
	Audience string

	server *httptest.Server
	priv   *rsa.PrivateKey
	keyID  string
}

// NewTestIssuer starts a test OIDC issuer. It is shut down via t.Cleanup.
func NewTestIssuer(t *testing.T) *TestIssuer {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	ti := &TestIssuer{priv: priv, keyID: "test-key", Audience: TestAudience}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   ti.URL,
			"jwks_uri": ti.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		pub, err := jwk.Import(priv.Public())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = pub.Set(jwk.KeyIDKey, ti.keyID)
		_ = pub.Set(jwk.AlgorithmKey, jwa.RS256())
		_ = pub.Set(jwk.KeyUsageKey, "sig")
		set := jwk.NewSet()
		_ = set.AddKey(pub)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	})

	ti.server = httptest.NewServer(mux)
	ti.URL = ti.server.URL
	t.Cleanup(ti.server.Close)
	return ti
}

// Token mints a signed ID token for the given subject, email and display
// name, valid for one hour.
func (ti *TestIssuer) Token(t *testing.T, subject, email, displayName string) string {
	t.Helper()

	now := time.Now()
	tok, err := jwt.NewBuilder().
		Issuer(ti.URL).
		Audience([]string{ti.Audience}).
		Subject(subject).
		IssuedAt(now).
		Expiration(now.Add(time.Hour)).
		Claim("email", email).
		Claim("name", displayName).
		Build()
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	key, err := jwk.Import(ti.priv)
	if err != nil {
		t.Fatalf("import key: %v", err)
	}
	_ = key.Set(jwk.KeyIDKey, ti.keyID)

	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), key))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return string(signed)
}
