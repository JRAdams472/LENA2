// Discord OAuth2 redirect helpers. The client never sees the client
// secret — it collects an authorization code and the server exchanges it.

export const DISCORD_STATE_KEY = "lena_discord_oauth_state";

export function discordEnabled(): boolean {
  return !!process.env.NEXT_PUBLIC_DISCORD_CLIENT_ID;
}

// discordAuthorizeUrl builds the authorization redirect. `state` is a
// caller-generated nonce stored in sessionStorage and verified on the
// callback — the standard OAuth CSRF guard.
export function discordAuthorizeUrl(state: string): string {
  const params = new URLSearchParams({
    client_id: process.env.NEXT_PUBLIC_DISCORD_CLIENT_ID ?? "",
    response_type: "code",
    redirect_uri:
      process.env.NEXT_PUBLIC_DISCORD_REDIRECT_URI ??
      `${window.location.origin}/auth/discord/callback`,
    scope: "identify email",
    state,
  });
  return `https://discord.com/oauth2/authorize?${params.toString()}`;
}
