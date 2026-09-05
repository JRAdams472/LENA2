"use client";

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useSyncExternalStore,
  useCallback,
  ReactNode,
  useRef,
} from "react";
import {
  setAuthTokenGetter,
  setOnUnauthorized,
} from "@/lib/api";

interface GoogleJwtPayload {
  email?: string;
  sub?: string;
  exp?: number;
}

export interface AuthUser {
  email: string;
  sub?: string;
}

interface AuthContextValue {
  token: string | null;
  user: AuthUser | null;
  isAuthenticated: boolean;
  signIn: (credential: string) => void;
  signOut: () => void;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

const TOKEN_KEY = "lena_id_token";

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

const tokenStore = (() => {
  const listeners = new Set<() => void>();
  return {
    subscribe(callback: () => void) {
      if (typeof window === "undefined") return () => {};
      const handler = (e: StorageEvent) => {
        if (e.key === TOKEN_KEY) callback();
      };
      window.addEventListener("storage", handler);
      listeners.add(callback);
      return () => {
        window.removeEventListener("storage", handler);
        listeners.delete(callback);
      };
    },
    getSnapshot(): string | null {
      if (typeof window === "undefined") return null;
      const stored = window.localStorage.getItem(TOKEN_KEY);
      if (!stored) return null;
      if (isTokenExpired(stored)) return null;
      return stored;
    },
    setToken(value: string | null) {
      if (typeof window === "undefined") return;
      if (value === null) {
        window.localStorage.removeItem(TOKEN_KEY);
      } else {
        window.localStorage.setItem(TOKEN_KEY, value);
      }
      listeners.forEach((cb) => cb());
    },
  };
})();

function getUserFromToken(token: string | null): AuthUser | null {
  if (!token) return null;
  const payload = decodeJwtPayload(token);
  return payload?.email ? { email: payload.email, sub: payload.sub } : null;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const token = useSyncExternalStore(
    tokenStore.subscribe,
    tokenStore.getSnapshot,
    () => null
  );
  const tokenRef = useRef<string | null>(token);
  const user = useMemo(() => getUserFromToken(token), [token]);

  const signIn = useCallback((credential: string) => {
    tokenRef.current = credential;
    tokenStore.setToken(credential);
  }, []);

  const signOut = useCallback(() => {
    tokenRef.current = null;
    tokenStore.setToken(null);
  }, []);

  useEffect(() => {
    setAuthTokenGetter(() => tokenRef.current);
    setOnUnauthorized(signOut);
  }, [signOut]);

  useEffect(() => {
    tokenRef.current = token;
  }, [token]);

  const value: AuthContextValue = {
    token,
    user,
    isAuthenticated: !!token && !!user,
    signIn,
    signOut,
  };

  return (
    <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
