"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { GoogleLogin } from "@react-oauth/google";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import { useAuth } from "@/app/auth/AuthProvider";

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
          LENA
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
        {error && (
          <Typography color="error" sx={{ mt: 1 }}>
            {error}
          </Typography>
        )}
      </Paper>
    </Box>
  );
}
