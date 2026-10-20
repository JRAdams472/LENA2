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

// Discord OAuth2 provider. Discord is not OIDC — there is no ID token to
// verify. The client collects an authorization code; the server exchanges
// it (holding client_secret) and then proves identity with
// GET /users/@me. Both calls happen once, at sign-in or link time — the
// resulting LENA session means requests never touch Discord again.

// discordAPIBase is the API root; tests substitute an httptest server.
const discordAPIBase = "https://discord.com/api/v10"

// errDiscordCredential is returned for any failure to prove identity via
// Discord (bad/expired/used code, exchange rejection, @me failure).
var errDiscordCredential = errors.New("discord credential rejected")

// DiscordVerifier exchanges authorization codes and resolves the Discord
// user. Nil when DiscordClientID/Secret are not configured.
type DiscordVerifier struct {
	clientID       string
	clientSecret   string
	redirectURI    string
	extraRedirects map[string]struct{}
	base           string
	http           *http.Client
}

// NewDiscordVerifier builds the verifier; returns nil when the provider
// is not configured so handlers can feature-detect it.
// extraRedirectURIs are additional redirect URIs the exchange accepts —
// the mobile app deep-links back under a custom scheme, so the client
// reports which registered URI its authorize request used.
func NewDiscordVerifier(clientID, clientSecret, redirectURI string, extraRedirectURIs ...string) *DiscordVerifier {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &DiscordVerifier{
		clientID:       clientID,
		clientSecret:   clientSecret,
		redirectURI:    redirectURI,
		extraRedirects: extraRedirectSet(extraRedirectURIs),
		base:           discordAPIBase,
		http:           &http.Client{Timeout: 10 * time.Second},
	}
}

// verify exchanges the authorization code and fetches /users/@me.
// Discord has no id_token, so the nonce parameter is ignored; the PKCE
// code_verifier is forwarded to the token exchange when present.
// redirectURI is the URI the client's authorize request used; it is
// validated against the registered set before the exchange.
func (d *DiscordVerifier) verify(ctx context.Context, code, _, codeVerifier, redirectURI string) (providerIdentity, error) {
	redirect, err := d.resolveRedirectURI(redirectURI)
	if err != nil {
		return providerIdentity{}, err
	}
	accessToken, err := d.exchange(ctx, code, codeVerifier, redirect)
	if err != nil {
		return providerIdentity{}, err
	}
	return d.fetchUser(ctx, accessToken)
}

// resolveRedirectURI picks the redirect URI to echo in the token
// exchange — see OAuthOIDCVerifier.resolveRedirectURI.
func (d *DiscordVerifier) resolveRedirectURI(requested string) (string, error) {
	if requested == "" || requested == d.redirectURI {
		return d.redirectURI, nil
	}
	if _, ok := d.extraRedirects[requested]; ok {
		return requested, nil
	}
	return "", fmt.Errorf("%w: redirect_uri is not registered", errDiscordCredential)
}

func (d *DiscordVerifier) exchange(ctx context.Context, code, codeVerifier, redirectURI string) (string, error) {
	form := url.Values{
		"client_id":     {d.clientID},
		"client_secret": {d.clientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.base+"/oauth2/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errDiscordCredential, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := d.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errProviderUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errProviderUnavailable, err)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("%w: decode token response: %w", errDiscordCredential, err)
	}
	if res.StatusCode != http.StatusOK || tok.AccessToken == "" {
		return "", fmt.Errorf("%w: exchange failed (%s)", errDiscordCredential, tok.Error)
	}
	return tok.AccessToken, nil
}

func (d *DiscordVerifier) fetchUser(ctx context.Context, accessToken string) (providerIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.base+"/users/@me", nil)
	if err != nil {
		return providerIdentity{}, fmt.Errorf("%w: %w", errDiscordCredential, err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	res, err := d.http.Do(req)
	if err != nil {
		return providerIdentity{}, fmt.Errorf("%w: %w", errProviderUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()

	var me struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
		Email      string `json:"email"`
		Verified   bool   `json:"verified"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&me); err != nil {
		return providerIdentity{}, fmt.Errorf("%w: decode user: %w", errDiscordCredential, err)
	}
	if res.StatusCode != http.StatusOK || me.ID == "" {
		return providerIdentity{}, fmt.Errorf("%w: users/@me returned %d", errDiscordCredential, res.StatusCode)
	}
	name := me.GlobalName
	if name == "" {
		name = me.Username
	}
	return providerIdentity{
		provider:      DiscordProvider,
		subject:       me.ID,
		email:         me.Email,
		emailVerified: me.Verified,
		name:          name,
	}, nil
}
