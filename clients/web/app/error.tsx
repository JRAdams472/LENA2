"use client";

import { useEffect } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";

// Root error boundary — without this file a render error falls through
// to Next's unstyled default. `reset` re-renders the segment.
export default function ErrorPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "60vh",
        gap: 2,
        px: 2,
        textAlign: "center",
      }}
    >
      <Typography variant="h2">Something went wrong</Typography>
      <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 400 }}>
        The page hit an unexpected error. Trying again usually clears it —
        if it keeps happening, the team will want the details below.
      </Typography>
      {error.digest && (
        <Typography variant="caption" color="text.secondary">
          Error reference: {error.digest}
        </Typography>
      )}
      <Button variant="contained" onClick={reset}>
        Try again
      </Button>
    </Box>
  );
}
