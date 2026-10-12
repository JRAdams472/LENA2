package bff

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// Sign in with Apple. Web clients run the standard authorization-code
// flow against appleid.apple.com; the code exchange is server-side
// because the client_secret is not a static string — Apple requires a
// short-lived ES256 JWT signed with the developer account's .p8 key.
// iOS clients sign in natively and send the returned identityToken as a
// bearer: the same issuer/JWKS path validates it under the bundle-ID
// audience.

// appleIssuer is the iss claim on Apple id_tokens (web code exchange and
// iOS native credentials alike) and the aud claim on client-secret JWTs.
const appleIssuer = "https://appleid.apple.com"

// appleExchangeURL is the authorization-code exchange URL.
const appleExchangeURL = "https://appleid.apple.com/auth/token"

// appleSecretTTL bounds the minted client_secret JWT. Apple accepts up
// to six months; minutes are enough — a fresh JWT is minted per
// exchange, so a captured secret expires almost immediately.
const appleSecretTTL = 5 * time.Minute

// AppleIssuer is the issuer claim on Apple id_tokens.
func AppleIssuer() string {
	return appleIssuer
}

// LoadApplePrivateKey reads a .p8 file (PEM PKCS8) into the ECDSA
// P-256 key Apple issues for Sign in with Apple.
func LoadApplePrivateKey(path string) (*ecdsa.PrivateKey, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path is the operator-configured APPLE_PRIVATE_KEY_FILE secret mount.
	if err != nil {
		return nil, fmt.Errorf("read apple private key: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("apple private key is not PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse apple private key: %w", err)
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok || ec.Curve != elliptic.P256() {
		return nil, errors.New("apple private key is not an ECDSA P-256 key")
	}
	return ec, nil
}

// appleClientSecret mints the ES256 JWT Apple expects as client_secret:
// iss = Team ID, sub = Services ID (client_id), aud = appleid.apple.com,
// header kid = Key ID. TTL is minutes — one JWT per exchange.
func appleClientSecret(teamID, keyID, clientID string, key *ecdsa.PrivateKey, now time.Time) (string, error) {
	jwkKey, err := jwk.Import(key)
	if err != nil {
		return "", fmt.Errorf("import apple key: %w", err)
	}
	if err := jwkKey.Set(jwk.KeyIDKey, keyID); err != nil {
		return "", fmt.Errorf("set apple key id: %w", err)
	}
	tok, err := jwt.NewBuilder().
		Issuer(teamID).
		Subject(clientID).
		Audience([]string{appleIssuer}).
		IssuedAt(now).
		Expiration(now.Add(appleSecretTTL)).
		Build()
	if err != nil {
		return "", fmt.Errorf("build apple client secret: %w", err)
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.ES256(), jwkKey))
	if err != nil {
		return "", fmt.Errorf("sign apple client secret: %w", err)
	}
	return string(signed), nil
}

// NewAppleVerifier builds a code-exchange verifier for Sign in with
// Apple. It returns nil unless every Apple config value is present so
// the provider registry simply lacks the key — sign-in and link
// requests then answer 503 like any unconfigured provider. Apple echoes
// the authorize-request nonce in the id_token, so it is required.
func NewAppleVerifier(clientID, teamID, keyID, redirectURI string, privateKey *ecdsa.PrivateKey, tokens oidcTokenVerifier, extraRedirectURIs ...string) *OAuthOIDCVerifier {
	if clientID == "" || teamID == "" || keyID == "" || privateKey == nil || tokens == nil {
		return nil
	}
	return &OAuthOIDCVerifier{
		clientID: clientID,
		clientSecretFn: func(context.Context) (string, error) {
			return appleClientSecret(teamID, keyID, clientID, privateKey, time.Now())
		},
		redirectURI:    redirectURI,
		extraRedirects: extraRedirectSet(extraRedirectURIs),
		tokenEndpoint:  appleExchangeURL,
		tokens:         tokens,
		requireNonce:   true,
		http:           &http.Client{Timeout: 10 * time.Second},
	}
}
