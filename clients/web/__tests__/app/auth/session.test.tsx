import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { AuthProvider, useAuth } from "@/app/auth/AuthProvider";

const mockFetch = global.fetch as jest.Mock;

function b64(obj: object): string {
  return btoa(JSON.stringify(obj))
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
}

function makeToken(claims: object): string {
  return `${b64({ alg: "none" })}.${b64(claims)}.sig`;
}

const future = Math.floor(Date.now() / 1000) + 3600;
const googleToken = makeToken({
  iss: "https://accounts.google.com",
  email: "test@example.com",
  sub: "g-sub",
  exp: future,
});
const lenaAccess = makeToken({ iss: "lena", sub: "7", exp: future });

function Probe() {
  const { isAuthenticated, isRestoring, user, signIn, signInWithProvider, signOut } =
    useAuth();
  return (
    <div>
      <span data-testid="auth">{String(isAuthenticated)}</span>
      <span data-testid="restoring">{String(isRestoring)}</span>
      <span data-testid="email">{user?.email ?? ""}</span>
      <button onClick={() => signIn(googleToken)}>in</button>
      <button
        onClick={() =>
          signInWithProvider("discord", "code-1", "nonce-1", "verifier-1").catch(
            () => undefined
          )
        }
      >
        discord
      </button>
      <button onClick={signOut}>out</button>
    </div>
  );
}

function sessionOk(refreshToken = "rt-1") {
  return {
    ok: true,
    status: 200,
    json: async () => ({
      accessToken: lenaAccess,
      refreshToken,
      expiresAt: new Date(Date.now() + 86400000).toISOString(),
    }),
  };
}

const meData = {
  id: "7",
  email: "restored@example.com",
  displayName: null,
  firstName: null,
  lastName: null,
  backupEmail: null,
  birthdate: null,
  role: "member",
  isActive: true,
  isProtected: false,
  lastLoginAt: null,
  isSearchable: false,
  household: null,
};

describe("AuthProvider sessions", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    localStorage.clear();
    sessionStorage.clear();
  });

  it("exchanges the provider credential for a session on sign-in", async () => {
    mockFetch.mockResolvedValue(sessionOk());

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );
    fireEvent.click(screen.getByText("in"));

    await waitFor(() => {
      expect(screen.getByTestId("auth").textContent).toBe("true");
    });

    const [url, init] = mockFetch.mock.calls[0];
    expect(url).toBe("http://localhost:5059/auth/session");
    expect((init as RequestInit).headers).toEqual(
      expect.objectContaining({ Authorization: `Bearer ${googleToken}` })
    );
    // The LENA access token becomes the bearer; the refresh credential
    // lives in the HttpOnly cookie so only the non-secret hint persists.
    expect(sessionStorage.getItem("lena_id_token")).toBe(lenaAccess);
    expect(localStorage.getItem("lena_session_hint")).toBe("1");
    expect(localStorage.getItem("lena_refresh_token")).toBeNull();
    expect(screen.getByTestId("email").textContent).toBe("test@example.com");
  });

  it("exchanges a Discord code and hydrates identity via me", async () => {
    mockFetch
      .mockResolvedValueOnce(sessionOk("rt-discord"))
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ data: { me: meData } }),
      });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );
    fireEvent.click(screen.getByText("discord"));

    await waitFor(() => {
      expect(screen.getByTestId("auth").textContent).toBe("true");
    });

    const [url, init] = mockFetch.mock.calls[0];
    expect(url).toBe("http://localhost:5059/auth/session/discord");
    expect(JSON.parse((init as RequestInit).body as string)).toEqual(
      expect.objectContaining({ code: "code-1", nonce: "nonce-1", codeVerifier: "verifier-1" })
    );
    expect(sessionStorage.getItem("lena_id_token")).toBe(lenaAccess);
    expect(localStorage.getItem("lena_session_hint")).toBe("1");
    await waitFor(() => {
      expect(screen.getByTestId("email").textContent).toBe(
        "restored@example.com"
      );
    });
  });

  it("rejects when the Discord exchange fails", async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );
    fireEvent.click(screen.getByText("discord"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalled();
    });
    expect(screen.getByTestId("auth").textContent).toBe("false");
    expect(localStorage.getItem("lena_session_hint")).toBeNull();
  });

  it("falls back to the provider credential when sessions are disabled", async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 503, text: async () => "" });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );
    fireEvent.click(screen.getByText("in"));

    await waitFor(() => {
      expect(screen.getByTestId("auth").textContent).toBe("true");
    });
    expect(sessionStorage.getItem("lena_id_token")).toBe(googleToken);
    expect(localStorage.getItem("lena_session_hint")).toBeNull();
  });

  it("restores a session from the session hint", async () => {
    localStorage.setItem("lena_session_hint", "1");
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/auth/session/refresh"))
        return Promise.resolve(sessionOk("rt-rotated"));
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ data: { me: meData } }),
      });
    });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );
    expect(screen.getByTestId("restoring").textContent).toBe("true");

    await waitFor(() => {
      expect(screen.getByTestId("auth").textContent).toBe("true");
    });
    // The refresh request carried no body token — the cookie supplies
    // the credential. The user snapshot was rebuilt via the me query.
    const refreshCall = mockFetch.mock.calls.find(([u]: [string]) =>
      u.includes("/auth/session/refresh")
    );
    expect(refreshCall).toBeDefined();
    expect(JSON.parse(refreshCall[1].body as string).refreshToken).toBe("");
    expect(localStorage.getItem("lena_session_hint")).toBe("1");
    expect(sessionStorage.getItem("lena_id_token")).toBe(lenaAccess);
    expect(screen.getByTestId("email").textContent).toBe("restored@example.com");
  });

  it("migrates a legacy localStorage refresh token", async () => {
    localStorage.setItem("lena_refresh_token", "rt-stored");
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/auth/session/refresh"))
        return Promise.resolve(sessionOk("rt-rotated"));
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ data: { me: meData } }),
      });
    });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId("auth").textContent).toBe("true");
    });
    // The legacy token was sent in the body for the final rotation, then
    // removed — the new credential exists only as the cookie.
    const refreshCall = mockFetch.mock.calls.find(([u]: [string]) =>
      u.includes("/auth/session/refresh")
    );
    expect(JSON.parse(refreshCall[1].body as string).refreshToken).toBe(
      "rt-stored"
    );
    expect(localStorage.getItem("lena_refresh_token")).toBeNull();
    expect(localStorage.getItem("lena_session_hint")).toBe("1");
  });

  it("clears a dead session and stays signed out", async () => {
    localStorage.setItem("lena_session_hint", "1");
    mockFetch.mockResolvedValue({ ok: false, status: 401, text: async () => "" });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId("restoring").textContent).toBe("false");
    });
    expect(screen.getByTestId("auth").textContent).toBe("false");
    expect(localStorage.getItem("lena_session_hint")).toBeNull();
  });

  it("revokes the session on sign-out", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/auth/session")) return Promise.resolve(sessionOk());
      return Promise.resolve({ ok: true, status: 204, json: async () => ({}) });
    });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );
    fireEvent.click(screen.getByText("in"));
    await waitFor(() => {
      expect(screen.getByTestId("auth").textContent).toBe("true");
    });

    fireEvent.click(screen.getByText("out"));

    await waitFor(() => {
      expect(screen.getByTestId("auth").textContent).toBe("false");
    });
    const revokeCall = mockFetch.mock.calls.find(([u]: [string]) =>
      u.includes("/auth/session/revoke")
    );
    expect(revokeCall).toBeDefined();
    expect(JSON.parse(revokeCall[1].body as string).refreshToken).toBe("");
    expect(localStorage.getItem("lena_session_hint")).toBeNull();
    expect(sessionStorage.getItem("lena_id_token")).toBeNull();
  });
});
