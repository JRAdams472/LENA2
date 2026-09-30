"use client";

import { Suspense, useCallback, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Typography from "@mui/material/Typography";
import { useAuth } from "@/app/auth/AuthProvider";
import { DISCORD_STATE_KEY } from "@/lib/discord";

// Discord redirects here with ?code=...&state=... after authorization.
function DiscordCallback() {
  const router = useRouter();
  const params = useSearchParams();
  const { signInWithDiscord, isAuthenticated } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const ranRef = useRef(false);

  const fail = useCallback(
    (msg: string) => void Promise.resolve().then(() => setError(msg)),
    []
  );

  useEffect(() => {
    if (ranRef.current) return;
    ranRef.current = true;

    const code = params.get("code");
    const state = params.get("state");
    const expected = window.sessionStorage.getItem(DISCORD_STATE_KEY);
    window.sessionStorage.removeItem(DISCORD_STATE_KEY);

    if (params.get("error")) {
      fail("Discord sign-in was cancelled or denied.");
      return;
    }
    if (!code || !state || !expected || state !== expected) {
      fail("Invalid Discord sign-in response. Please try again.");
      return;
    }

    signInWithDiscord(code)
      .then(() => router.push("/"))
      .catch(() => setError("Discord sign-in failed. Please try again."));
  }, [params, signInWithDiscord, router, fail]);

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
      {error ? (
        <Typography color="error">{error}</Typography>
      ) : (
        <>
          <CircularProgress />
          <Typography color="text.secondary">
            Completing Discord sign-in…
          </Typography>
        </>
      )}
    </Box>
  );
}

export default function DiscordCallbackPage() {
  return (
    <Suspense>
      <DiscordCallback />
    </Suspense>
  );
}
