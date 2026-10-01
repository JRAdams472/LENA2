"use client";

import { Suspense, useCallback, useEffect, useRef, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Typography from "@mui/material/Typography";
import { useAuth } from "@/app/auth/AuthProvider";
import {
  isOAuthProvider,
  oauthNonceKey,
  oauthStateKey,
} from "@/lib/oauth";

// OAuth providers redirect here with ?code=...&state=... after
// authorization; the provider segment selects the exchange endpoint.
function OAuthCallback() {
  const router = useRouter();
  const params = useSearchParams();
  const routeParams = useParams<{ provider: string }>();
  const { signInWithProvider, isAuthenticated } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const ranRef = useRef(false);

  const fail = useCallback(
    (msg: string) => void Promise.resolve().then(() => setError(msg)),
    []
  );

  const provider = isOAuthProvider(routeParams.provider)
    ? routeParams.provider
    : null;

  useEffect(() => {
    if (ranRef.current || provider === null) return;
    ranRef.current = true;

    const code = params.get("code");
    const state = params.get("state");
    const expected = window.sessionStorage.getItem(oauthStateKey(provider));
    const nonce =
      window.sessionStorage.getItem(oauthNonceKey(provider)) ?? undefined;
    window.sessionStorage.removeItem(oauthStateKey(provider));
    window.sessionStorage.removeItem(oauthNonceKey(provider));

    if (params.get("error")) {
      fail("Sign-in was cancelled or denied.");
      return;
    }
    if (!code || !state || !expected || state !== expected) {
      fail("Invalid sign-in response. Please try again.");
      return;
    }

    signInWithProvider(provider, code, nonce)
      .then(() => router.push("/"))
      .catch(() => setError("Sign-in failed. Please try again."));
  }, [provider, params, signInWithProvider, router, fail]);

  useEffect(() => {
    if (isAuthenticated) router.push("/");
  }, [isAuthenticated, router]);

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "100vh",
        gap: 2,
      }}
    >
      {error || provider === null ? (
        <Typography color="error">
          {error ?? "Unknown sign-in provider."}
        </Typography>
      ) : (
        <>
          <CircularProgress />
          <Typography color="text.secondary">Completing sign-in…</Typography>
        </>
      )}
    </Box>
  );
}

export default function OAuthCallbackPage() {
  return (
    <Suspense>
      <OAuthCallback />
    </Suspense>
  );
}
