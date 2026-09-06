package main

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

func TestEnv(t *testing.T) {
	t.Setenv("TESTISSUER_TEST_KEY", "")
	if got := env("TESTISSUER_TEST_KEY", "fallback"); got != "fallback" {
		t.Fatalf("got %q, want fallback", got)
	}
	t.Setenv("TESTISSUER_TEST_KEY", "value")
	if got := env("TESTISSUER_TEST_KEY", "fallback"); got != "value" {
		t.Fatalf("got %q, want value", got)
	}
}

func testMux(t *testing.T) (*httptest.Server, jwk.Key, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pub, err := jwk.Import(priv.Public())
	if err != nil {
		t.Fatalf("import public key: %v", err)
	}
	_ = pub.Set(jwk.KeyIDKey, "e2e-key")
	_ = pub.Set(jwk.AlgorithmKey, jwa.RS256())
	jwks := jwk.NewSet()
	_ = jwks.AddKey(pub)
	signingKey, err := jwk.Import(priv)
	if err != nil {
		t.Fatalf("import private key: %v", err)
	}
	_ = signingKey.Set(jwk.KeyIDKey, "e2e-key")

	srv := httptest.NewServer(nil)
	srv.Config.Handler = newMux(srv.URL, "lena-e2e-client", jwks, signingKey)
	t.Cleanup(srv.Close)
	return srv, signingKey, srv.URL
}

func TestDiscoveryAndHealth(t *testing.T) {
	srv, _, issuer := testMux(t)

	resp, err := http.Get(srv.URL + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	defer resp.Body.Close()
	var doc struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("decode discovery: %v", err)
	}
	if doc.Issuer != issuer || doc.JWKSURI != issuer+"/jwks" {
		t.Fatalf("unexpected discovery doc: %+v", doc)
	}

	resp, err = http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status %d", resp.StatusCode)
	}
}

func TestTokenEndpoint(t *testing.T) {
	srv, _, issuer := testMux(t)

	resp, err := http.Get(srv.URL + "/token?sub=sub-42&email=e2e@example.com&name=E2E&aud=custom-aud")
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if body.IDToken == "" {
		t.Fatal("empty id_token")
	}

	// Verify the minted token against the served JWKS.
	keySet, err := jwk.Fetch(t.Context(), srv.URL+"/jwks")
	if err != nil {
		t.Fatalf("fetch jwks: %v", err)
	}
	tok, err := jwt.Parse([]byte(body.IDToken), jwt.WithKeySet(keySet), jwt.WithValidate(true))
	if err != nil {
		t.Fatalf("verify minted token: %v", err)
	}
	if iss, _ := tok.Issuer(); iss != issuer {
		t.Fatalf("issuer %q, want %q", iss, issuer)
	}
	if sub, _ := tok.Subject(); sub != "sub-42" {
		t.Fatalf("subject %q", sub)
	}
	var email string
	if err := tok.Get("email", &email); err != nil || email != "e2e@example.com" {
		t.Fatalf("email claim %q err=%v", email, err)
	}
	if exp, ok := tok.Expiration(); !ok || exp.Before(time.Now()) {
		t.Fatal("token not valid for an hour")
	}
}

func TestTokenDefaults(t *testing.T) {
	srv, _, _ := testMux(t)

	resp, err := http.Get(srv.URL + "/token")
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tok, err := jwt.ParseInsecure([]byte(body.IDToken))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if sub, _ := tok.Subject(); sub != "e2e-subject" {
		t.Fatalf("default subject %q", sub)
	}
	if aud, _ := tok.Audience(); len(aud) != 1 || aud[0] != "lena-e2e-client" {
		t.Fatalf("default audience %v", aud)
	}
}
