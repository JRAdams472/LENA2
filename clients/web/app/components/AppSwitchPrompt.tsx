"use client";

import { useState } from "react";
import Alert from "@mui/material/Alert";
import IconButton from "@mui/material/IconButton";
import Snackbar from "@mui/material/Snackbar";
import CloseIcon from "@mui/icons-material/Close";
import SmartphoneIcon from "@mui/icons-material/Smartphone";
import useMediaQuery from "@mui/material/useMediaQuery";
import { useTheme } from "@mui/material/styles";

const DISMISSED_KEY = "lena_app_prompt_dismissed";

// One-time nudge toward the Flutter app for members browsing on a phone.
// Member chrome only — the marketing site never shows this (there is no
// marketing app), and dismissal is remembered per device in localStorage
// so it can never become a recurring nag.
// TODO: link the Play Store / install instructions once the app is published.
export default function AppSwitchPrompt() {
  const theme = useTheme();
  const isPhone = useMediaQuery(theme.breakpoints.down("md"), { noSsr: true });
  const [dismissed, setDismissed] = useState(
    () =>
      typeof window !== "undefined" &&
      window.localStorage.getItem(DISMISSED_KEY) === "1"
  );

  if (!isPhone || dismissed) return null;

  const dismiss = () => {
    window.localStorage.setItem(DISMISSED_KEY, "1");
    setDismissed(true);
  };

  return (
    <Snackbar
      open
      anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      sx={{ mb: 1 }}
    >
      <Alert
        severity="info"
        icon={<SmartphoneIcon />}
        variant="filled"
        data-testid="app-switch-prompt"
        action={
          <IconButton
            size="small"
            color="inherit"
            aria-label="Dismiss"
            onClick={dismiss}
          >
            <CloseIcon fontSize="small" />
          </IconButton>
        }
      >
        Using LENA on a phone? The mobile app is built for shopping, cook
        mode, and pantry scanning — everything here syncs with it.
      </Alert>
    </Snackbar>
  );
}
