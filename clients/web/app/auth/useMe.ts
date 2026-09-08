"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { User } from "@/lib/types";
import { useAuth } from "./AuthProvider";

interface MeState {
  me: User | null;
  isLoading: boolean;
  error: Error | null;
}

/**
 * Fetches the current user's profile (including server-side role) once per
 * sign-in. Role comes from the API — never the JWT — so admin gating in
 * the UI matches what the backend enforces. Deliberately does not use
 * react-query: AdminLayout renders outside QueryClientProvider in tests.
 */
export function useMe() {
  const { isAuthenticated, token } = useAuth();
  const [state, setState] = useState<MeState>({
    me: null,
    isLoading: isAuthenticated,
    error: null,
  });
  const requestId = useRef(0);

  const load = useCallback(() => {
    const id = ++requestId.current;
    api
      .getMe()
      .then((u) => {
        if (requestId.current === id)
          setState({ me: u, isLoading: false, error: null });
      })
      .catch((e: Error) => {
        if (requestId.current === id)
          setState({ me: null, isLoading: false, error: e });
      });
  }, []);

  useEffect(() => {
    if (isAuthenticated && token) {
      load();
    }
  }, [isAuthenticated, token, load]);

  return {
    me: isAuthenticated ? state.me : null,
    isAdmin: isAuthenticated && state.me?.role === "admin",
    isLoading: isAuthenticated ? state.isLoading : false,
    error: state.error,
    refetch: load,
  };
}
