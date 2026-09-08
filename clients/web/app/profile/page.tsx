"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Paper from "@mui/material/Paper";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { api, ApiError } from "@/lib/api";
import { User } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";

// Keyed by userID so the form mounts with the loaded profile values and
// remounts if the signed-in user changes — avoids syncing props into state.
function ProfileForm({ me, onSaved }: { me: User; onSaved: () => void }) {
  const [firstName, setFirstName] = useState(me.firstName ?? "");
  const [lastName, setLastName] = useState(me.lastName ?? "");
  const [backupEmail, setBackupEmail] = useState(me.backupEmail ?? "");
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: () =>
      api.updateMyProfile({ firstName, lastName, backupEmail }),
    onSuccess: () => {
      setSaved(true);
      setError(null);
      onSaved();
    },
    onError: (e) => {
      setSaved(false);
      setError(e instanceof ApiError ? e.message : "Failed to save profile");
    },
  });

  return (
    <Box
      component="form"
      onSubmit={(e) => {
        e.preventDefault();
        mutation.mutate();
      }}
      sx={{ display: "flex", flexDirection: "column", gap: 2 }}
    >
      {saved && <Alert severity="success">Profile saved.</Alert>}
      {error && <Alert severity="error">{error}</Alert>}
      <TextField
        label="First name"
        value={firstName}
        onChange={(e) => setFirstName(e.target.value)}
        slotProps={{ htmlInput: { maxLength: 100 } }}
        fullWidth
      />
      <TextField
        label="Last name"
        value={lastName}
        onChange={(e) => setLastName(e.target.value)}
        slotProps={{ htmlInput: { maxLength: 100 } }}
        fullWidth
      />
      <TextField
        label="Backup email"
        type="email"
        value={backupEmail}
        onChange={(e) => setBackupEmail(e.target.value)}
        helperText="Used only if we need to reach you and your sign-in email fails."
        fullWidth
      />
      <Button type="submit" variant="contained" disabled={mutation.isPending}>
        {mutation.isPending ? "Saving…" : "Save"}
      </Button>
    </Box>
  );
}

export default function ProfilePage() {
  const { me, isLoading, refetch } = useMe();

  if (isLoading || !me) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Paper sx={{ maxWidth: 480, p: 3 }}>
      <Typography variant="h5" gutterBottom>
        Profile
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        {me.email} — {me.role === "admin" ? "Administrator" : "Member"}
      </Typography>
      <ProfileForm key={me.userID} me={me} onSaved={refetch} />
    </Paper>
  );
}
