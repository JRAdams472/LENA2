"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { GoogleLogin } from "@react-oauth/google";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Paper from "@mui/material/Paper";
import SvgIcon from "@mui/material/SvgIcon";
import Typography from "@mui/material/Typography";
import { useAuth } from "@/app/auth/AuthProvider";
import LenaLogo from "@/app/components/LenaLogo";
import {
  DISCORD_STATE_KEY,
  discordAuthorizeUrl,
  discordEnabled,
} from "@/lib/discord";

// Discord brand mark (blurple #5865F2 is applied via the button).
function DiscordIcon() {
  return (
    <SvgIcon viewBox="0 0 24 24">
      <path d="M20.317 4.37a19.79 19.79 0 00-4.885-1.515.074.074 0 00-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 00-5.487 0 12.64 12.64 0 00-.617-1.25.077.077 0 00-.079-.037A19.736 19.736 0 003.677 4.37a.07.07 0 00-.032.027C.533 9.046-.32 13.58.099 18.058a.082.082 0 00.031.056 19.9 19.9 0 005.993 3.03.078.078 0 00.084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 00-.041-.106 13.107 13.107 0 01-1.872-.892.077.077 0 01-.008-.128c.126-.094.252-.192.372-.291a.074.074 0 01.077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 01.078.009c.12.099.246.198.373.292a.077.077 0 01-.006.127 12.3 12.3 0 01-1.873.892.077.077 0 00-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 00.084.028 19.84 19.84 0 006.002-3.03.077.077 0 00.032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 00-.031-.03zM8.02 15.33c-1.182 0-2.157-1.085-2.157-2.419 0-1.333.955-2.418 2.157-2.418 1.21 0 2.176 1.095 2.157 2.418 0 1.334-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.418 2.157-2.418 1.21 0 2.176 1.095 2.157 2.418 0 1.334-.946 2.418-2.157 2.418z" />
    </SvgIcon>
  );
}

function startDiscordSignIn() {
  const state = crypto.randomUUID();
  window.sessionStorage.setItem(DISCORD_STATE_KEY, state);
  window.location.assign(discordAuthorizeUrl(state));
}

export default function LoginScreen() {
  const { signIn, isAuthenticated } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();

  useEffect(() => {
    if (isAuthenticated) {
      router.push("/");
    }
  }, [isAuthenticated, router]);

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "100vh",
        px: 2,
      }}
    >
      <Paper
        elevation={3}
        sx={{
          p: 4,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          gap: 3,
          maxWidth: 400,
          width: "100%",
          textAlign: "center",
        }}
      >
        <Typography variant="h4" component="h1">
          <LenaLogo size={36} />
        </Typography>
        <Typography color="text.secondary">
          Sign in to manage inventory, recipes, and meal plans.
        </Typography>
        <GoogleLogin
          onSuccess={(response) => {
            setError(null);
            if (response.credential) {
              signIn(response.credential);
            } else {
              setError(
                "Google did not return a sign-in credential. Please try again."
              );
            }
          }}
          onError={() => {
            setError(
              "Google sign-in failed. Verify NEXT_PUBLIC_GOOGLE_CLIENT_ID and the authorized JavaScript origin in Google Cloud Console."
            );
          }}
        />
        {discordEnabled() && (
          <Button
            variant="contained"
            startIcon={<DiscordIcon />}
            onClick={startDiscordSignIn}
            sx={{
              bgcolor: "#5865F2",
              "&:hover": { bgcolor: "#4752C4" },
              textTransform: "none",
            }}
          >
            Sign in with Discord
          </Button>
        )}
        {error && (
          <Typography color="error" sx={{ mt: 1 }}>
            {error}
          </Typography>
        )}
      </Paper>
    </Box>
  );
}
