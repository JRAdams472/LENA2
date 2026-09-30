"use client";

import {
  createContext,
  use,
  useEffect,
  useSyncExternalStore,
  useCallback,
  useState,
  ReactNode,
  useRef,
} from "react";
import {
  api,
  setAuthTokenGetter,
  setOnUnauthorized,
  setSessionRefresher,
  createDiscordSession,
  createSession,
  refreshSessionRequest,
  revokeSession,
} from "@/lib/api";

interface GoogleJwtPayload {
  email?: string;
  sub?: string;
  exp?: number;
  iss?: string;
}

export interface AuthUser {
  email: string;
  sub?: string;
}

interface AuthContextValue {
  token: string | null;
  user: AuthUser | null;
  isAuthenticated: boolean;
  // isRestoring is true while a stored refresh token is being exchanged
  // (new tab / cleared tab storage) — SilentReAuth stays quiet meanwhile.
  isRestoring: boolean;
  signIn: (credential: string) => void;
  // signInWithDiscord exchanges an OAuth2 authorization code for a LENA
  // session. Rejects when the exchange fails — no OIDC fallback exists.
  signInWithDiscord: (code: string) => Promise<void>;
  signOut: () => void;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

const TOKEN_KEY = "lena_id_token"; // sessionStorage — the active bearer
const REFRESH_KEY = "lena_refresh_token"; // localStorage — survives tabs
const USER_KEY = "lena_auth_user"; // sessionStorage — {email, sub} snapshot

const DEVICE = "web";

function decodeJwtPayload(token: string): GoogleJwtPayload | null {
  try {
    const payload = token.split(".")[1];
    const base64 = payload.replace(/-/g, "+").replace(/_/g, "/");
    const padding = "=".repeat((4 - (base64.length % 4)) % 4);
    const json = atob(base64 + padding);
    return JSON.parse(json) as GoogleJwtPayload;
  } catch {
    return null;
  }
}

function isTokenExpired(token: string): boolean {
  const payload = decodeJwtPayload(token);
  if (typeof payload?.exp !== "number") return false;
  return payload.exp < Math.floor(Date.now() / 1000);
}

function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(REFRESH_KEY);
}

function setRefreshToken(value: string | null) {
  if (typeof window === "undefined") return;
  if (value === null) {
    window.localStorage.removeItem(REFRESH_KEY);
  } else {
    window.localStorage.setItem(REFRESH_KEY, value);
  }
}

function getStoredUser(): AuthUser | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.sessionStorage.getItem(USER_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as AuthUser;
    return parsed?.email ? parsed : null;
  } catch {
    return null;
  }
}

// persistUser stores the display identity decoded from a provider
// credential — LENA access tokens carry only the user id, so the session
// keeps the email/name snapshot from the sign-in token.
function persistUser(user: AuthUser | null) {
  if (typeof window === "undefined") return;
  if (user === null) {
    window.sessionStorage.removeItem(USER_KEY);
  } else {
    window.sessionStorage.setItem(USER_KEY, JSON.stringify(user));
  }
}

const tokenStore = (() => {
  const listeners = new Set<() => void>();
  return {
    subscribe(callback: () => void) {
      // sessionStorage is per-tab, so there are no cross-tab "storage"
      // events to relay — subscribers only need intra-tab notifications,
      // which setToken delivers directly.
      listeners.add(callback);
      return () => {
        listeners.delete(callback);
      };
    },
    getSnapshot(): string | null {
      if (typeof window === "undefined") return null;
      // Migrate tokens persisted by older versions (and by the e2e
      // storage-state seed) from localStorage into the per-tab store,
      // then remove the persistent copy.
      const legacy = window.localStorage.getItem(TOKEN_KEY);
      if (legacy) {
        window.sessionStorage.setItem(TOKEN_KEY, legacy);
        window.localStorage.removeItem(TOKEN_KEY);
      }
      const stored = window.sessionStorage.getItem(TOKEN_KEY);
      if (!stored) return null;
      // An expired access token is still returned when a refresh token
      // exists — the first request 401s, refreshes, and retries rather
      // than bouncing to Google sign-in.
      if (isTokenExpired(stored) && !getRefreshToken()) return null;
      return stored;
    },
    setToken(value: string | null) {
      if (typeof window === "undefined") return;
      if (value === null) {
        window.sessionStorage.removeItem(TOKEN_KEY);
        window.localStorage.removeItem(TOKEN_KEY);
      } else {
        window.sessionStorage.setItem(TOKEN_KEY, value);
      }
      listeners.forEach((cb) => cb());
    },
  };
})();

function getUserForToken(token: string | null): AuthUser | null {
  if (!token) return null;
  const payload = decodeJwtPayload(token);
  // Provider ID tokens carry the email directly; LENA access tokens use
  // the sign-in snapshot stored alongside the session.
  if (payload?.email) return { email: payload.email, sub: payload.sub };
  return getStoredUser();
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const token = useSyncExternalStore(
    tokenStore.subscribe,
    tokenStore.getSnapshot,
    () => null
  );
  const tokenRef = useRef<string | null>(token);
  const [isRestoring, setIsRestoring] = useState(
    () => !token && !!getRefreshToken()
  );
  // The display identity normally derives from the token or its stored
  // snapshot; a restored session may hydrate it via the me query *after*
  // the access token lands, so an override covers that late arrival.
  const [userOverride, setUserOverride] = useState<AuthUser | null>(null);
  const user = getUserForToken(token) ?? userOverride;

  // One refresh rotation; also the handler registered with api.ts for
  // the 401 → refresh → retry path. User info survives via USER_KEY;
  // when it is missing (fresh tab) the me query rehydrates it.
  const refreshSessionNow = useCallback(async (): Promise<boolean> => {
    const rt = getRefreshToken();
    if (!rt) return false;
    const bundle = await refreshSessionRequest(rt, DEVICE).catch(() => null);
    if (!bundle) return false;
    setRefreshToken(bundle.refreshToken);
    tokenStore.setToken(bundle.accessToken);
    if (!getStoredUser()) {
      const me = await api.getMe().catch(() => null);
      if (me?.email) {
        const u = { email: me.email, sub: String(me.userID) };
        persistUser(u);
        setUserOverride(u);
      }
    }
    return true;
  }, []);

  const signIn = useCallback((credential: string) => {
    const userInfo = (() => {
      const payload = decodeJwtPayload(credential);
      return payload?.email ? { email: payload.email, sub: payload.sub } : null;
    })();
    // Exchange the provider credential for a LENA session; on any
    // failure (sessions disabled, network, server error) the credential
    // itself stays the bearer — OIDC-only mode keeps working.
    createSession(credential, DEVICE)
      .then((bundle) => {
        persistUser(userInfo);
        if (bundle) {
          setRefreshToken(bundle.refreshToken);
          tokenStore.setToken(bundle.accessToken);
        } else {
          tokenStore.setToken(credential);
        }
      })
      .catch(() => {
        persistUser(userInfo);
        tokenStore.setToken(credential);
      });
  }, []);

  // Discord sign-in: the authorization code exchanges for a session
  // server-side, then `me` hydrates the display identity (a code carries
  // no readable claims).
  const signInWithDiscord = useCallback(async (code: string) => {
    const bundle = await createDiscordSession(code, DEVICE);
    setRefreshToken(bundle.refreshToken);
    tokenStore.setToken(bundle.accessToken);
    const me = await api.getMe().catch(() => null);
    if (me?.email) {
      const u = { email: me.email, sub: String(me.userID) };
      persistUser(u);
      setUserOverride(u);
    }
  }, []);

  const signOut = useCallback(() => {
    const rt = getRefreshToken();
    if (rt) {
      setRefreshToken(null);
      void revokeSession(rt);
    }
    persistUser(null);
    setUserOverride(null);
    tokenRef.current = null;
    tokenStore.setToken(null);
  }, []);

  useEffect(() => {
    // Read storage on each call so requests fired before the store
    // subscription settles still send the persisted token.
    setAuthTokenGetter(() => tokenStore.getSnapshot());
    setOnUnauthorized(signOut);
    setSessionRefresher(refreshSessionNow);
  }, [signOut, refreshSessionNow]);

  // New tab (or cleared tab storage) with a live refresh token: rotate
  // once to restore the session instead of forcing Google re-auth.
  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current) return;
    restoredRef.current = true;
    // isRestoring initialized true only when this condition is false, so
    // the early return never needs to clear it.
    if (tokenStore.getSnapshot() || !getRefreshToken()) return;
    void refreshSessionNow().then((ok) => {
      if (!ok) setRefreshToken(null);
      setIsRestoring(false);
    });
  }, [refreshSessionNow]);

  useEffect(() => {
    tokenRef.current = token;
  }, [token]);

  const value: AuthContextValue = {
    token,
    user,
    isAuthenticated: !!token && !!user,
    isRestoring,
    signIn,
    signInWithDiscord,
    signOut,
  };

  return <AuthContext value={value}>{children}</AuthContext>;
}

export function useAuth(): AuthContextValue {
  const context = use(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
