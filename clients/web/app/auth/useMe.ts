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
  const requestIdRef = useRef(0);
  // Post-mount hint so returning admins see the Administration nav group
  // on the first frame instead of it popping in after the /me round-trip.
  // Read in an effect (not the useState initializer) to avoid an SSR/CSR
  // hydration mismatch; the real role always wins once the fetch lands.
  const [adminHint, setAdminHint] = useState(false);
  useEffect(() => {
    setAdminHint(localStorage.getItem("lena_is_admin") === "1");
  }, []);

  const load = useCallback(() => {
    const id = ++requestIdRef.current;
    api
      .getMe()
      .then((u) => {
        if (requestIdRef.current === id) {
          localStorage.setItem(
            "lena_is_admin",
            u.role === "admin" ? "1" : "0"
          );
          setAdminHint(u.role === "admin");
          setState({ me: u, isLoading: false, error: null });
        }
      })
      .catch((e: Error) => {
        if (requestIdRef.current === id)
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
    isAdmin:
      isAuthenticated &&
      (state.me?.role === "admin" || (state.isLoading && adminHint)),
    isLoading: isAuthenticated ? state.isLoading : false,
    error: state.error,
    refetch: load,
  };
}
