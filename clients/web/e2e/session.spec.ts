import { test, expect, APIRequestContext } from "@playwright/test";
import { graphql, mintToken, SECOND_USER } from "./helpers";

// Session endpoints are unauthenticated (the refresh token is the
// credential), so these tests run without the stored auth state.
test.use({ storageState: { cookies: [], origins: [] } });

const BASE = process.env.E2E_BASE_URL ?? "http://localhost";

interface SessionBundle {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
}

async function createSession(
  request: APIRequestContext,
  idToken: string
): Promise<SessionBundle> {
  const res = await request.post(`${BASE}/auth/session`, {
    headers: { Authorization: `Bearer ${idToken}` },
    data: { device: "e2e" },
  });
  expect(res.status()).toBe(200);
  return (await res.json()) as SessionBundle;
}

async function refresh(
  request: APIRequestContext,
  refreshToken: string
): Promise<number> {
  const res = await request.post(`${BASE}/auth/session/refresh`, {
    data: { refreshToken },
  });
  return res.status();
}

async function refreshOk(
  request: APIRequestContext,
  refreshToken: string
): Promise<SessionBundle> {
  const res = await request.post(`${BASE}/auth/session/refresh`, {
    data: { refreshToken },
  });
  expect(res.status()).toBe(200);
  return (await res.json()) as SessionBundle;
}

test.describe("session lifecycle", () => {
  test("create → authenticate → refresh rotation → reuse revocation", async ({
    request,
  }) => {
    const idToken = await mintToken(request, SECOND_USER);

    // Provider credential exchanges for a LENA session.
    const s1 = await createSession(request, idToken);
    expect(s1.accessToken).toBeTruthy();
    expect(s1.refreshToken).toBeTruthy();

    // The LENA access token authenticates GraphQL (iss=lena path).
    const me = await graphql<{ me: { email: string } }>(
      request,
      s1.accessToken,
      `query { me { email } }`
    );
    expect(me.me.email).toBe(SECOND_USER.email);

    // A session access token cannot mint a new session.
    const reuseOfAccess = await request.post(`${BASE}/auth/session`, {
      headers: { Authorization: `Bearer ${s1.accessToken}` },
      data: {},
    });
    expect(reuseOfAccess.status()).toBe(403);

    // Rotation: a fresh pair in the same family.
    const s2 = await refreshOk(request, s1.refreshToken);
    expect(s2.refreshToken).not.toBe(s1.refreshToken);
    expect(s2.accessToken).toBeTruthy();

    // Replaying the rotated token is theft: 401 and the whole family dies.
    expect(await refresh(request, s1.refreshToken)).toBe(401);
    expect(await refresh(request, s2.refreshToken)).toBe(401);
    // Note: the in-flight access token stays cryptographically valid until
    // its ~15-minute expiry — revocation bounds the refresh side, which is
    // the persistence channel.
  });

  test("revoke ends the session", async ({ request }) => {
    const idToken = await mintToken(request, SECOND_USER);
    const s = await createSession(request, idToken);

    const res = await request.post(`${BASE}/auth/session/revoke`, {
      data: { refreshToken: s.refreshToken },
    });
    expect(res.status()).toBe(204);

    expect(await refresh(request, s.refreshToken)).toBe(401);
    expect(await refresh(request, "bogus-token")).toBe(401);
  });
});
