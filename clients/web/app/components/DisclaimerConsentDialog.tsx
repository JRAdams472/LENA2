"use client";

import { useState } from "react";
import Link from "next/link";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import { api, ApiError } from "@/lib/api";

// LEN-31: the one-time consent gate for recording allergies/dietary
// restrictions. Self-contained — owns the acceptDisclaimer call so any
// surface can mount it and react via onAccepted.
export default function DisclaimerConsentDialog({
  open,
  onClose,
  onAccepted,
}: {
  open: boolean;
  onClose: () => void;
  onAccepted: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const accept = async () => {
    setBusy(true);
    setError(null);
    try {
      await api.acceptDisclaimer();
      onAccepted();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not record acceptance");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onClose={busy ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Medical &amp; allergen disclaimer</DialogTitle>
      <DialogContent>
        <DialogContentText component="div" sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <span>
            Allergen and dietary information in Lena&apos;s Kitchen is a
            convenience, not medical, nutritional, or dietary advice. Records
            may be incomplete, inaccurate, or out of date — always check the
            product&apos;s actual label, and consult a qualified healthcare
            professional about allergies or medical conditions.
          </span>
          <span>
            By accepting you acknowledge this and use these features at your own
            risk. Read the full{" "}
            <Link href="/disclaimer" target="_blank">
              medical &amp; allergen disclaimer
            </Link>
            .
          </span>
        </DialogContentText>
        {error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={busy}>
          Not now
        </Button>
        <Button
          onClick={accept}
          variant="contained"
          disabled={busy}
          data-testid="disclaimer-accept"
        >
          {busy ? "Accepting…" : "I understand and accept"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
