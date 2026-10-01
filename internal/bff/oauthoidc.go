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
// code exchange returns an OIDC id_token (Microsoft Entra, Facebook).
// The exchange needs the client_secret, so it happens server-side; the
// resulting id_token is verified by the same issuer/JWKS/audience
// machinery as a client-supplied credential — no separate trust path.
type OAuthOIDCVerifier struct {
	clientID      string
	clientSecret  string
	redirectURI   string
	tokenEndpoint string
	tokens        oidcTokenVerifier
	requireNonce  bool
	http          *http.Client
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
func NewOAuthOIDCVerifier(clientID, clientSecret, redirectURI, tokenEndpoint string, tokens oidcTokenVerifier, requireNonce bool) *OAuthOIDCVerifier {
	if clientID == "" || clientSecret == "" || tokens == nil {
		return nil
	}
	return &OAuthOIDCVerifier{
		clientID:      clientID,
		clientSecret:  clientSecret,
		redirectURI:   redirectURI,
		tokenEndpoint: tokenEndpoint,
		tokens:        tokens,
		requireNonce:  requireNonce,
		http:          &http.Client{Timeout: 10 * time.Second},
	}
}

// verify exchanges the code, verifies the returned id_token, and checks
// the nonce when the flow uses one. The provider key is the token's
// issuer claim — identical to what a bearer-token sign-in produces.
func (v *OAuthOIDCVerifier) verify(ctx context.Context, code, nonce string) (providerIdentity, error) {
	if v.requireNonce && nonce == "" {
		return providerIdentity{}, fmt.Errorf("%w: nonce is required", errOAuthCredential)
	}
	raw, err := v.exchange(ctx, code)
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
// endpoint and returns the id_token field of the response.
func (v *OAuthOIDCVerifier) exchange(ctx context.Context, code string) (string, error) {
	form := url.Values{
		"client_id":     {v.clientID},
		"client_secret": {v.clientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {v.redirectURI},
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
