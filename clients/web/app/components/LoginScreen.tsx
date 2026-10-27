"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Fade from "@mui/material/Fade";
import Link from "@mui/material/Link";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import { useAuth } from "@/app/auth/AuthProvider";
import LenaLogo from "@/app/components/LenaLogo";
import ProviderButtons from "@/app/components/ProviderButtons";

export default function LoginScreen() {
  const { isAuthenticated } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (isAuthenticated) {
      router.push("/member");
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
      <Fade in timeout={400}>
      <Paper
        elevation={3}
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", sm: "5fr 6fr" },
          maxWidth: 720,
          width: "100%",
          overflow: "hidden",
        }}
      >
        {/* Brand panel — the signature sage band, same in both color modes
            (deep sage keeps the cream wordmark at AA). */}
        <Box
          sx={{
            bgcolor: "#5F7A57",
            color: "#FFFDF8",
            p: { xs: 4, sm: 5 },
            display: "flex",
            flexDirection: "column",
            justifyContent: "center",
            gap: 1.5,
          }}
        >
          <Typography component="h1" variant="h4" sx={{ lineHeight: 1 }}>
            <LenaLogo size={40} iconColor="#FFFDF8" textColor="#FFFDF8" />
          </Typography>
          <Typography
            variant="body2"
            sx={{ color: "rgba(255,253,248,0.85)", mt: 1 }}
          >
            The household kitchen, kept — pantry, recipes, meal plans, and wine
            in one place.
          </Typography>
        </Box>

        <Box
          sx={{
            p: { xs: 4, sm: 5 },
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 2,
            textAlign: "center",
          }}
        >
        <Typography variant="h6">Welcome to Lena</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: -1 }}>
          Sign in with the provider associated with your web account to
          access your dashboard and configure notifications.
        </Typography>
        <ProviderButtons />
        {/* Login-only by design (LEN-136): account creation happens on
            the marketing site, never through this flow. The domain is a
            link — a plain-text URL is a dead end for new users (LEN-180). */}
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ maxWidth: 340 }}
        >
          Account registration and plan management are handled on our
          website. New users must set up their account at{" "}
          <Link href="https://lenaskitchen.app" target="_blank" rel="noopener">
            lenaskitchen.app
          </Link>{" "}
          before signing in here or in the mobile app.
        </Typography>
        </Box>
      </Paper>
      </Fade>
    </Box>
  );
}
