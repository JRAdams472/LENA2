import {
  authorizeUrl,
  enabledProviders,
  isOAuthProvider,
  oauthNonceKey,
  oauthStateKey,
  providerEnabled,
} from "@/lib/oauth";

const ENV_KEYS = [
  "NEXT_PUBLIC_DISCORD_CLIENT_ID",
  "NEXT_PUBLIC_DISCORD_REDIRECT_URI",
  "NEXT_PUBLIC_MICROSOFT_CLIENT_ID",
  "NEXT_PUBLIC_MICROSOFT_TENANT",
  "NEXT_PUBLIC_MICROSOFT_REDIRECT_URI",
  "NEXT_PUBLIC_FACEBOOK_CLIENT_ID",
  "NEXT_PUBLIC_FACEBOOK_REDIRECT_URI",
] as const;

describe("oauth providers", () => {
  const saved: Record<string, string | undefined> = {};
  beforeEach(() => {
    for (const k of ENV_KEYS) {
      saved[k] = process.env[k];
      delete process.env[k];
    }
  });
  afterEach(() => {
    for (const k of ENV_KEYS) {
      if (saved[k] === undefined) delete process.env[k];
      else process.env[k] = saved[k];
    }
  });

  it("detects enabled providers from env", () => {
    expect(enabledProviders()).toEqual([]);
    process.env.NEXT_PUBLIC_DISCORD_CLIENT_ID = "d-id";
    process.env.NEXT_PUBLIC_FACEBOOK_CLIENT_ID = "f-id";
    expect(enabledProviders()).toEqual(["discord", "facebook"]);
    expect(providerEnabled("microsoft")).toBe(false);
  });

  it("validates provider names for the dynamic callback route", () => {
    expect(isOAuthProvider("discord")).toBe(true);
    expect(isOAuthProvider("microsoft")).toBe(true);
    expect(isOAuthProvider("facebook")).toBe(true);
    expect(isOAuthProvider("refresh")).toBe(false);
    expect(isOAuthProvider("../evil")).toBe(false);
  });

  it("builds the discord authorize url", () => {
    process.env.NEXT_PUBLIC_DISCORD_CLIENT_ID = "d-id";
    const url = new URL(authorizeUrl("discord", "st", "nn"));
    expect(url.origin + url.pathname).toBe(
      "https://discord.com/oauth2/authorize"
    );
    expect(url.searchParams.get("client_id")).toBe("d-id");
    expect(url.searchParams.get("state")).toBe("st");
    expect(url.searchParams.get("scope")).toBe("identify email");
    expect(url.searchParams.get("redirect_uri")).toBe(
      "http://localhost/auth/discord/callback"
    );
  });

  it("builds the microsoft authorize url with nonce + tenant", () => {
    process.env.NEXT_PUBLIC_MICROSOFT_CLIENT_ID = "m-id";
    const url = new URL(authorizeUrl("microsoft", "st", "nn"));
    expect(url.origin + url.pathname).toBe(
      "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize"
    );
    expect(url.searchParams.get("client_id")).toBe("m-id");
    expect(url.searchParams.get("nonce")).toBe("nn");
    expect(url.searchParams.get("scope")).toBe("openid profile email");
    expect(url.searchParams.get("redirect_uri")).toBe(
      "http://localhost/auth/microsoft/callback"
    );
  });

  it("honors a configured microsoft tenant", () => {
    process.env.NEXT_PUBLIC_MICROSOFT_CLIENT_ID = "m-id";
    process.env.NEXT_PUBLIC_MICROSOFT_TENANT = "my-tenant-guid";
    const url = new URL(authorizeUrl("microsoft", "st", "nn"));
    expect(url.pathname).toContain("/my-tenant-guid/");
  });

  it("builds the facebook authorize url with nonce", () => {
    process.env.NEXT_PUBLIC_FACEBOOK_CLIENT_ID = "f-id";
    const url = new URL(authorizeUrl("facebook", "st", "nn"));
    expect(url.origin + url.pathname).toBe(
      "https://www.facebook.com/v21.0/dialog/oauth"
    );
    expect(url.searchParams.get("client_id")).toBe("f-id");
    expect(url.searchParams.get("nonce")).toBe("nn");
    expect(url.searchParams.get("scope")).toBe("openid email");
  });

  it("namespaces state and nonce keys per provider", () => {
    expect(oauthStateKey("discord")).not.toBe(oauthStateKey("facebook"));
    expect(oauthNonceKey("microsoft")).toBe("lena_oauth_nonce_microsoft");
  });
});
