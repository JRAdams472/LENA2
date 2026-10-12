package bff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthOIDCVerifier proves identity for providers whose authorization-
// code exchange returns an OIDC id_token (Microsoft Entra, Facebook,
// Apple). The exchange needs the client_secret, so it happens
// server-side; the resulting id_token is verified by the same
// issuer/JWKS/audience machinery as a client-supplied credential — no
// separate trust path.
type OAuthOIDCVerifier struct {
	clientID     string
	clientSecret string
	// clientSecretFn mints the client_secret per exchange when the
	// provider rejects static secrets (Apple requires a short-lived
	// ES256 JWT signed with the developer .p8 key). Nil → static.
	clientSecretFn func(ctx context.Context) (string, error)
	redirectURI    string
	extraRedirects map[string]struct{}
	tokenEndpoint  string
	tokens         oidcTokenVerifier
	requireNonce   bool
	http           *http.Client
}

// oidcTokenVerifier is the slice of *Authenticator the exchange
// verifier needs: verify an id_token, get the claims back.
type oidcTokenVerifier interface {
	verifyOIDCToken(ctx context.Context, raw string) (oidcClaims, error)
}

// errOAuthCredential covers any failure in the exchange or id_token
// verification — a bad code, a rejected exchange, a nonce mismatch.
var errOAuthCredential = errors.New("oauth credential rejected")

// NewOAuthOIDCVerifier builds a verifier for a code-exchange OIDC
// provider. It returns nil when the provider is unconfigured so the
// registry simply lacks the key. tokenEndpoint is the provider's
// authorization-code exchange URL; issuer is documented for clarity but
// the trusted issuer is whatever the verified token claims.
// extraRedirectURIs are additional redirect URIs the exchange accepts
// beyond redirectURI — mobile clients deep-link back into the app under
// a custom scheme, so the client tells the server which URI it used and
// only a registered one is honored.
func NewOAuthOIDCVerifier(clientID, clientSecret, redirectURI, tokenEndpoint string, tokens oidcTokenVerifier, requireNonce bool, extraRedirectURIs ...string) *OAuthOIDCVerifier {
	if clientID == "" || clientSecret == "" || tokens == nil {
		return nil
	}
	return &OAuthOIDCVerifier{
		clientID:       clientID,
		clientSecret:   clientSecret,
		redirectURI:    redirectURI,
		extraRedirects: extraRedirectSet(extraRedirectURIs),
		tokenEndpoint:  tokenEndpoint,
		tokens:         tokens,
		requireNonce:   requireNonce,
		http:           &http.Client{Timeout: 10 * time.Second},
	}
}

// extraRedirectSet indexes the allowlisted redirect URIs beyond the
// configured web one; empty entries are dropped.
func extraRedirectSet(uris []string) map[string]struct{} {
	if len(uris) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(uris))
	for _, uri := range uris {
		if uri != "" {
			m[uri] = struct{}{}
		}
	}
	return m
}

// resolveRedirectURI picks the redirect URI to echo in the token
// exchange: the configured web URI when the request omits one, or the
// requested value when it is registered (web URI or an allowlisted
// mobile URI). Anything else is rejected — the exchange must never echo
// an unregistered URI.
func (v *OAuthOIDCVerifier) resolveRedirectURI(requested string) (string, error) {
	if requested == "" || requested == v.redirectURI {
		return v.redirectURI, nil
	}
	if _, ok := v.extraRedirects[requested]; ok {
		return requested, nil
	}
	return "", fmt.Errorf("%w: redirect_uri is not registered", errOAuthCredential)
}

// verify exchanges the code, verifies the returned id_token, and checks
// the nonce when the flow uses one. The provider key is the token's
// issuer claim — identical to what a bearer-token sign-in produces.
// redirectURI is the URI the client's authorize request used; it is
// validated against the registered set before the exchange.
func (v *OAuthOIDCVerifier) verify(ctx context.Context, code, nonce, codeVerifier, redirectURI string) (providerIdentity, error) {
	if v.requireNonce && nonce == "" {
		return providerIdentity{}, fmt.Errorf("%w: nonce is required", errOAuthCredential)
	}
	redirect, err := v.resolveRedirectURI(redirectURI)
	if err != nil {
		return providerIdentity{}, err
	}
	raw, err := v.exchange(ctx, code, codeVerifier, redirect)
	if err != nil {
		return providerIdentity{}, err
	}
	claims, err := v.tokens.verifyOIDCToken(ctx, raw)
	if err != nil {
		return providerIdentity{}, fmt.Errorf("%w: %w", errOAuthCredential, err)
	}
	if nonce != "" && claims.nonce != nonce {
		return providerIdentity{}, fmt.Errorf("%w: nonce mismatch", errOAuthCredential)
	}
	return providerIdentity{
		provider:      claims.issuer,
		subject:       claims.subject,
		email:         claims.email,
		emailVerified: claims.emailVerified,
		name:          claims.name,
	}, nil
}

// exchange posts the authorization code to the provider's token
// endpoint and returns the id_token field of the response. A non-empty
// codeVerifier is forwarded as the PKCE code_verifier parameter.
func (v *OAuthOIDCVerifier) exchange(ctx context.Context, code, codeVerifier, redirectURI string) (string, error) {
	secret := v.clientSecret
	if v.clientSecretFn != nil {
		var err error
		secret, err = v.clientSecretFn(ctx)
		if err != nil {
			return "", fmt.Errorf("%w: mint client secret: %w", errOAuthCredential, err)
		}
	}
	form := url.Values{
		"client_id":     {v.clientID},
		"client_secret": {secret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.tokenEndpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errOAuthCredential, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := v.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errOAuthCredential, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errOAuthCredential, err)
	}

	var tok struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("%w: decode token response: %w", errOAuthCredential, err)
	}
	if res.StatusCode != http.StatusOK || tok.IDToken == "" {
		return "", fmt.Errorf("%w: exchange failed (%s)", errOAuthCredential, tok.Error)
	}
	return tok.IDToken, nil
}
