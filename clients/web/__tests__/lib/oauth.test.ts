import {
  authorizeUrl,
  deriveCodeChallenge,
  enabledProviders,
  generateCodeVerifier,
  isOAuthProvider,
  oauthNonceKey,
  oauthStateKey,
  oauthVerifierKey,
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
    const url = new URL(authorizeUrl("discord", "st", "nn", "ch"));
    expect(url.origin + url.pathname).toBe(
      "https://discord.com/oauth2/authorize"
    );
    expect(url.searchParams.get("client_id")).toBe("d-id");
    expect(url.searchParams.get("state")).toBe("st");
    expect(url.searchParams.get("scope")).toBe("identify email");
    expect(url.searchParams.get("redirect_uri")).toBe(
      "http://localhost/auth/discord/callback"
    );
    expect(url.searchParams.get("code_challenge")).toBe("ch");
    expect(url.searchParams.get("code_challenge_method")).toBe("S256");
  });

  it("builds the microsoft authorize url with nonce + tenant", () => {
    process.env.NEXT_PUBLIC_MICROSOFT_CLIENT_ID = "m-id";
    const url = new URL(authorizeUrl("microsoft", "st", "nn", "ch"));
    expect(url.origin + url.pathname).toBe(
      "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize"
    );
    expect(url.searchParams.get("client_id")).toBe("m-id");
    expect(url.searchParams.get("nonce")).toBe("nn");
    expect(url.searchParams.get("scope")).toBe("openid profile email");
    expect(url.searchParams.get("redirect_uri")).toBe(
      "http://localhost/auth/microsoft/callback"
    );
    expect(url.searchParams.get("code_challenge")).toBe("ch");
    expect(url.searchParams.get("code_challenge_method")).toBe("S256");
  });

  it("honors a configured microsoft tenant", () => {
    process.env.NEXT_PUBLIC_MICROSOFT_CLIENT_ID = "m-id";
    process.env.NEXT_PUBLIC_MICROSOFT_TENANT = "my-tenant-guid";
    const url = new URL(authorizeUrl("microsoft", "st", "nn", "ch"));
    expect(url.pathname).toContain("/my-tenant-guid/");
  });

  it("builds the facebook authorize url with nonce", () => {
    process.env.NEXT_PUBLIC_FACEBOOK_CLIENT_ID = "f-id";
    const url = new URL(authorizeUrl("facebook", "st", "nn", "ch"));
    expect(url.origin + url.pathname).toBe(
      "https://www.facebook.com/v21.0/dialog/oauth"
    );
    expect(url.searchParams.get("client_id")).toBe("f-id");
    expect(url.searchParams.get("nonce")).toBe("nn");
    expect(url.searchParams.get("scope")).toBe("openid email");
    expect(url.searchParams.get("code_challenge")).toBe("ch");
    expect(url.searchParams.get("code_challenge_method")).toBe("S256");
  });

  it("namespaces state and nonce keys per provider", () => {
    expect(oauthStateKey("discord")).not.toBe(oauthStateKey("facebook"));
    expect(oauthNonceKey("microsoft")).toBe("lena_oauth_nonce_microsoft");
    expect(oauthVerifierKey("discord")).toBe("lena_oauth_verifier_discord");
  });
});

describe("pkce", () => {
  it("generates a 43-char base64url verifier", () => {
    const v = generateCodeVerifier();
    expect(v).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(generateCodeVerifier()).not.toBe(v);
  });

  it("derives the RFC 7636 S256 challenge", async () => {
    // jsdom lacks crypto.subtle and TextEncoder — back them with Node's.
    const { webcrypto } = await import("node:crypto");
    const { TextEncoder } = await import("node:util");
    if (!globalThis.crypto?.subtle) {
      Object.defineProperty(globalThis, "crypto", { value: webcrypto });
    }
    if (globalThis.TextEncoder === undefined) {
      Object.defineProperty(globalThis, "TextEncoder", { value: TextEncoder });
    }
    // Appendix B vector: the challenge must be base64url(SHA-256(verifier)).
    const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk";
    await expect(deriveCodeChallenge(verifier)).resolves.toBe(
      "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
    );
  });
});
