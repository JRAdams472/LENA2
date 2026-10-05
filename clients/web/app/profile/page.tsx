"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import FormControlLabel from "@mui/material/FormControlLabel";
import Paper from "@mui/material/Paper";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import Typography from "@mui/material/Typography";
import { api, ApiError } from "@/lib/api";
import { MemberAllergyKind, User } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";

// Keyed by userID so the form mounts with the loaded profile values and
// remounts if the signed-in user changes — avoids syncing props into state.
function ProfileForm({ me, onSaved }: { me: User; onSaved: () => void }) {
  const [firstName, setFirstName] = useState(me.firstName ?? "");
  const [lastName, setLastName] = useState(me.lastName ?? "");
  const [backupEmail, setBackupEmail] = useState(me.backupEmail ?? "");
  const [birthdate, setBirthdate] = useState(me.birthdate ?? "");
  const [isSearchable, setIsSearchable] = useState(me.isSearchable);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: () =>
      api.updateMyProfile({ firstName, lastName, backupEmail, birthdate, isSearchable }),
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
      <TextField
        label="Birthdate"
        type="date"
        value={birthdate}
        onChange={(e) => setBirthdate(e.target.value)}
        helperText="Optional — unlocks wine pairing and cocktail suggestions (21+)."
        slotProps={{ inputLabel: { shrink: true } }}
        fullWidth
      />
      <FormControlLabel
        control={
          <Switch
            checked={isSearchable}
            onChange={(e) => setIsSearchable(e.target.checked)}
          />
        }
        label="Let other users find me by name or email to invite me to a household"
      />
      <Button type="submit" variant="contained" disabled={mutation.isPending}>
        {mutation.isPending ? "Saving…" : "Save"}
      </Button>
    </Box>
  );
}

// The caller's own allergy/dietary records — one per allergen. Each row
// is tri-state: no record, dietary preference, or allergy. Warnings on
// recipes/lists fire for any household member's records.
function AllergyCard() {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const allergensQ = useQuery({ queryKey: ["allergens"], queryFn: () => api.getAllergens() });
  const mineQ = useQuery({ queryKey: ["myAllergies"], queryFn: () => api.getMyAllergies() });

  const mutation = useMutation({
    mutationFn: ({ allergenID, kind, on }: { allergenID: number; kind: MemberAllergyKind; on: boolean }) =>
      api.setMyAllergy(allergenID, kind, on),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["myAllergies"] });
      // Warnings are computed per-entity; refresh anything that shows them.
      queryClient.invalidateQueries();
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : "Failed to update allergy records"),
  });

  if (allergensQ.isLoading || mineQ.isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", p: 2 }}>
        <CircularProgress size={24} />
      </Box>
    );
  }

  const mine = new Map((mineQ.data ?? []).map((m) => [m.allergen.allergenID, m.kind]));
  const allergens = (allergensQ.data ?? []).filter((a) => a.isActive);

  const onChange = (allergenID: number, value: string | null) => {
    const current = mine.get(allergenID);
    if (value === "none") {
      if (current) mutation.mutate({ allergenID, kind: current, on: false });
      return;
    }
    if (value === "allergy" || value === "dietary") {
      mutation.mutate({ allergenID, kind: value, on: true });
    }
  };

  return (
    <>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Mark allergies and dietary restrictions. Recipes, meal plans, events, and grocery
        lists warn when something conflicts with anyone in your household.
      </Typography>
      {allergens.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          No allergens are registered yet.
        </Typography>
      )}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        {allergens.map((a) => {
          const value = mine.get(a.allergenID) ?? "none";
          return (
            <Box
              key={a.allergenID}
              sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2 }}
            >
              <Typography variant="body1" title={a.description ?? undefined}>
                {a.name}
              </Typography>
              <ToggleButtonGroup
                size="small"
                exclusive
                value={value}
                onChange={(_e, v) => onChange(a.allergenID, v)}
                disabled={mutation.isPending}
                data-testid={`allergy-kind-${a.allergenID}`}
              >
                <ToggleButton value="none">None</ToggleButton>
                <ToggleButton value="dietary">Dietary</ToggleButton>
                <ToggleButton value="allergy" color="error">
                  Allergy
                </ToggleButton>
              </ToggleButtonGroup>
            </Box>
          );
        })}
      </Box>
    </>
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
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2, maxWidth: 520, mx: "auto" }}>
      <Paper sx={{ p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Profile
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {me.email} — {me.role === "admin" ? "Administrator" : "Member"}
        </Typography>
        <ProfileForm key={me.userID} me={me} onSaved={refetch} />
      </Paper>
      <Paper sx={{ p: 3 }}>
        <Typography variant="h6" gutterBottom>
          Allergies &amp; dietary restrictions
        </Typography>
        <AllergyCard />
      </Paper>
    </Box>
  );
}
