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

export function oauthVerifierKey(provider: OAuthProvider): string {
  return `lena_oauth_verifier_${provider}`;
}

// PKCE (RFC 7636): the browser keeps a high-entropy verifier, sends
// only its SHA-256 challenge to the provider, and hands the verifier
// to the BFF at exchange time — an intercepted code alone is useless
// (LEN-29 finding 4).
function base64url(bytes: Uint8Array): string {
  let bin = "";
  for (const b of bytes) bin += String.fromCodePoint(b);
  return btoa(bin).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
}

// generateCodeVerifier returns a 43-character base64url verifier — the
// shortest form RFC 7636 §4.1 allows, from 32 bytes of entropy.
export function generateCodeVerifier(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return base64url(bytes);
}

// deriveCodeChallenge computes the S256 challenge for a verifier.
export async function deriveCodeChallenge(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(verifier)
  );
  return base64url(new Uint8Array(digest));
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
// id_token to this browser flow. `challenge` is the S256 PKCE
// challenge — providers that don't implement PKCE ignore the pair.
export function authorizeUrl(
  provider: OAuthProvider,
  state: string,
  nonce: string,
  challenge: string
): string {
  const pkce = {
    code_challenge: challenge,
    code_challenge_method: "S256",
  };
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
        ...pkce,
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
        ...pkce,
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
        ...pkce,
      }).toString()}`;
  }
}

// startOAuthSignIn stores the CSRF state (+ nonce + PKCE verifier) and
// navigates to the provider's authorize page. Async because the S256
// challenge is a subtle-crypto digest.
export async function startOAuthSignIn(provider: OAuthProvider): Promise<void> {
  const state = crypto.randomUUID();
  const nonce = crypto.randomUUID();
  const verifier = generateCodeVerifier();
  const challenge = await deriveCodeChallenge(verifier);
  window.sessionStorage.setItem(oauthStateKey(provider), state);
  window.sessionStorage.setItem(oauthNonceKey(provider), nonce);
  window.sessionStorage.setItem(oauthVerifierKey(provider), verifier);
  window.location.assign(authorizeUrl(provider, state, nonce, challenge));
}
