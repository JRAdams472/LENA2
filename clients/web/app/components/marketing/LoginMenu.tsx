"use client";

import { useRef, useState } from "react";
import Button from "@mui/material/Button";
import Paper from "@mui/material/Paper";
import Popover from "@mui/material/Popover";
import Typography from "@mui/material/Typography";
import { useTheme } from "@mui/material/styles";
import { paletteFor } from "@/lib/themeVars";
import ProviderButtons from "@/app/components/ProviderButtons";

// The marketing site's sign-in affordance: a paper button on the sage
// header that opens the shared provider buttons in a popover. Successful
// sign-in flips useAuth; the landing page then routes to /member.
export default function LoginMenu() {
  const theme = useTheme();
  const anchorRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);

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
          <Typography variant="h6">Welcome back</Typography>
          <ProviderButtons />
        </Paper>
      </Popover>
    </>
  );
}
