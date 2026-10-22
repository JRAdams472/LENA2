"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Button from "@mui/material/Button";
import Paper from "@mui/material/Paper";
import Popover from "@mui/material/Popover";
import Typography from "@mui/material/Typography";
import { useTheme } from "@mui/material/styles";
import { paletteFor } from "@/lib/themeVars";
import { useAuth } from "@/app/auth/AuthProvider";
import ProviderButtons from "@/app/components/ProviderButtons";

// The marketing site's sign-in affordance: a paper button on the sage
// header that opens the shared provider buttons in a popover. Successful
// sign-in flips useAuth and routes to the member app.
export default function LoginMenu() {
  const theme = useTheme();
  const router = useRouter();
  const { isAuthenticated, isRestoring } = useAuth();
  const anchorRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  // The baseline is seeded on the first effect, not the first render:
  // useSyncExternalStore serves the null server snapshot through
  // hydration, so render-time isAuthenticated is always false — a member
  // would look like a fresh sign-in and get bounced to /member.
  const wasAuthedRef = useRef<boolean | null>(null);

  // Route to the member app only when auth flips during this session —
  // a signed-in member can still read the public docs without being
  // bounced back out.
  useEffect(() => {
    if (wasAuthedRef.current === null) {
      wasAuthedRef.current = isAuthenticated || isRestoring;
      return;
    }
    if (!wasAuthedRef.current && isAuthenticated) router.replace("/member");
    wasAuthedRef.current = isAuthenticated;
  }, [isAuthenticated, isRestoring, router]);

  return (
    <>
      <Button
        ref={anchorRef}
        variant="contained"
        disableElevation
        onClick={() => setOpen(true)}
        sx={{
          bgcolor: paletteFor(theme).primary.contrastText,
          color: paletteFor(theme).primary.dark,
          textTransform: "none",
          fontWeight: 700,
          "&:hover": {
            bgcolor: `color-mix(in srgb, ${paletteFor(theme).primary.contrastText} 88%, transparent)`,
          },
          "&:active": { transform: "translateY(1px)" },
        }}
      >
        Log in
      </Button>
      <Popover
        open={open}
        onClose={() => setOpen(false)}
        anchorEl={anchorRef.current}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
      >
        <Paper
          elevation={0}
          sx={{
            p: 3,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 2,
            minWidth: 300,
          }}
        >
          <Typography variant="h6">Welcome to Lena</Typography>
          <ProviderButtons />
        </Paper>
      </Popover>
    </>
  );
}
