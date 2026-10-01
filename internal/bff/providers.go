package bff

import (
	"context"
	"strings"
)

// Provider keys used on the code-exchange endpoints and stored on
// user_login rows for non-OIDC providers. OIDC-backed providers store
// the issuer URL as the key instead — the same row a bearer-token
// sign-in would produce, so a user is one account however they arrive.
const (
	DiscordProvider   = "discord"
	MicrosoftProvider = "microsoft"
	FacebookProvider  = "facebook"
)

// facebookIssuer is the iss claim on Facebook OIDC id_tokens.
const facebookIssuer = "https://www.facebook.com"

// microsoftConsumersTenantID is Microsoft's well-known constant for the
// "consumers" (personal account) tenant — tokens issued to MSA users
// carry it in their iss claim.
const microsoftConsumersTenantID = "9188040d-6c67-4c5b-b112-36a304b66dad"

// providerIdentity is what any code-exchange verifier proves: the
// provider key (login discriminator or OIDC issuer), stable subject,
// and whatever profile claims the provider returned.
type providerIdentity struct {
	provider      string
	subject       string
	email         string
	emailVerified bool
	name          string
}

// CodeVerifier exchanges an OAuth2 authorization code for the provider
// identity it proves. nonce is the value the client sent in its
// authorize request; providers that echo it in the id_token have it
// verified, others ignore it.
type CodeVerifier interface {
	verify(ctx context.Context, code, nonce string) (providerIdentity, error)
}

// MicrosoftIssuer maps a configured tenant to the issuer claim Entra
// puts in its id_tokens. "consumers" uses the well-known MSA tenant
// GUID; a tenant GUID issues under its own path. "common" and
// "organizations" are unsupported — their issuer is per-request
// templated, so no single iss value can be allowlisted.
func MicrosoftIssuer(tenant string) string {
	switch t := strings.ToLower(strings.TrimSpace(tenant)); t {
	case "consumers":
		return "https://login.microsoftonline.com/" + microsoftConsumersTenantID + "/v2.0"
	case "common", "organizations", "":
		return ""
	default:
		return "https://login.microsoftonline.com/" + t + "/v2.0"
	}
}

// FacebookIssuer is the issuer claim on Facebook OIDC id_tokens.
func FacebookIssuer() string {
	return facebookIssuer
}

// TrustIssuer appends an issuer/audience pair to the OIDC allowlist
// unless the issuer is already present (explicit AUTH_ISSUERS config
// wins). Provider issuers registered this way need no duplicate env
// config — setting the provider's client id/secret is sufficient.
func TrustIssuer(issuers, audiences []string, issuer, audience string) ([]string, []string) {
	if issuer == "" || audience == "" {
		return issuers, audiences
	}
	for _, iss := range issuers {
		if iss == issuer {
			return issuers, audiences
		}
	}
	return append(issuers, issuer), append(audiences, audience)
}
