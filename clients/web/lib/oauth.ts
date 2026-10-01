// OAuth2 provider redirect helpers. Clients never see provider secrets —
// they collect an authorization code (plus a nonce for OIDC providers)
// and the server exchanges it at /auth/session/{provider}.

export type OAuthProvider = "discord" | "microsoft" | "facebook";

export const OAUTH_PROVIDERS: OAuthProvider[] = [
  "discord",
  "microsoft",
  "facebook",
];

export function isOAuthProvider(v: string): v is OAuthProvider {
  return (OAUTH_PROVIDERS as string[]).includes(v);
}

export function oauthStateKey(provider: OAuthProvider): string {
  return `lena_oauth_state_${provider}`;
}

export function oauthNonceKey(provider: OAuthProvider): string {
  return `lena_oauth_nonce_${provider}`;
}

// redirectUri falls back to the same-origin callback route when the
// explicit env var is unset — it must match the server-side value the
// provider app has registered.
function redirectUri(envVar: string | undefined, provider: OAuthProvider): string {
  return envVar ?? `${window.location.origin}/auth/${provider}/callback`;
}

export function providerEnabled(provider: OAuthProvider): boolean {
  switch (provider) {
    case "discord":
      return !!process.env.NEXT_PUBLIC_DISCORD_CLIENT_ID;
    case "microsoft":
      return !!process.env.NEXT_PUBLIC_MICROSOFT_CLIENT_ID;
    case "facebook":
      return !!process.env.NEXT_PUBLIC_FACEBOOK_CLIENT_ID;
  }
}

export function enabledProviders(): OAuthProvider[] {
  return OAUTH_PROVIDERS.filter(providerEnabled);
}

// authorizeUrl builds the provider's authorization redirect. `state`
// and (for OIDC providers) `nonce` are caller-generated values stored
// in sessionStorage; state is the CSRF guard, nonce binds the returned
// id_token to this browser flow.
export function authorizeUrl(
  provider: OAuthProvider,
  state: string,
  nonce: string
): string {
  switch (provider) {
    case "discord":
      return `https://discord.com/oauth2/authorize?${new URLSearchParams({
        client_id: process.env.NEXT_PUBLIC_DISCORD_CLIENT_ID ?? "",
        response_type: "code",
        redirect_uri: redirectUri(
          process.env.NEXT_PUBLIC_DISCORD_REDIRECT_URI,
          provider
        ),
        scope: "identify email",
        state,
      }).toString()}`;
    case "microsoft":
      return `https://login.microsoftonline.com/${
        process.env.NEXT_PUBLIC_MICROSOFT_TENANT ?? "consumers"
      }/oauth2/v2.0/authorize?${new URLSearchParams({
        client_id: process.env.NEXT_PUBLIC_MICROSOFT_CLIENT_ID ?? "",
        response_type: "code",
        response_mode: "query",
        redirect_uri: redirectUri(
          process.env.NEXT_PUBLIC_MICROSOFT_REDIRECT_URI,
          provider
        ),
        scope: "openid profile email",
        state,
        nonce,
      }).toString()}`;
    case "facebook":
      return `https://www.facebook.com/v21.0/dialog/oauth?${new URLSearchParams({
        client_id: process.env.NEXT_PUBLIC_FACEBOOK_CLIENT_ID ?? "",
        response_type: "code",
        redirect_uri: redirectUri(
          process.env.NEXT_PUBLIC_FACEBOOK_REDIRECT_URI,
          provider
        ),
        scope: "openid email",
        state,
        nonce,
      }).toString()}`;
  }
}

// startOAuthSignIn stores the CSRF state (+ nonce) and navigates to the
// provider's authorize page.
export function startOAuthSignIn(provider: OAuthProvider): void {
  const state = crypto.randomUUID();
  const nonce = crypto.randomUUID();
  window.sessionStorage.setItem(oauthStateKey(provider), state);
  window.sessionStorage.setItem(oauthNonceKey(provider), nonce);
  window.location.assign(authorizeUrl(provider, state, nonce));
}
