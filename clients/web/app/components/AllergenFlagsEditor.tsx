"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Autocomplete from "@mui/material/Autocomplete";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import IconButton from "@mui/material/IconButton";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import DeleteIcon from "@mui/icons-material/Delete";
import { api } from "@/lib/api";
import { Allergen, AllergenFlag, AllergenFlagKind } from "@/lib/types";

// Admin curation for one entity's allergen flags. Each change is an
// immediate setIngredientAllergen/setItemAllergen call (kind=null clears)
// — the dialog's own Save only covers the entity's scalar fields.
// Local state mirrors the flags so edits show without a refetch; mount
// with key={entityID} so state resets per entity.
export default function AllergenFlagsEditor({
  flags,
  onSet,
  disabledReason,
}: {
  flags?: AllergenFlag[];
  onSet: (allergenID: number, kind: AllergenFlagKind | null) => Promise<unknown>;
  disabledReason?: string;
}) {
  const [local, setLocal] = useState<AllergenFlag[]>(flags ?? []);
  const [candidate, setCandidate] = useState<Allergen | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const allergensQuery = useQuery({ queryKey: ["allergens"], queryFn: () => api.getAllergens() });

  const apply = async (allergenID: number, kind: AllergenFlagKind | null) => {
    setPending(true);
    setError(null);
    try {
      await onSet(allergenID, kind);
      setLocal((prev) =>
        kind === null
          ? prev.filter((f) => f.allergen.allergenID !== allergenID)
          : [
              ...prev.filter((f) => f.allergen.allergenID !== allergenID),
              { allergen: { allergenID, name: nameOf(allergenID), description: null, isActive: true }, kind },
            ]
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update allergen flag");
    } finally {
      setPending(false);
    }
  };

  const nameOf = (allergenID: number) =>
    local.find((f) => f.allergen.allergenID === allergenID)?.allergen.name ??
    (allergensQuery.data ?? []).find((a) => a.allergenID === allergenID)?.name ??
    `Allergen ${allergenID}`;

  const unflagged = (allergensQuery.data ?? []).filter(
    (a) => a.isActive && !local.some((f) => f.allergen.allergenID === a.allergenID)
  );

  if (disabledReason) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
        {disabledReason}
      </Typography>
    );
  }

  return (
    <Box sx={{ mt: 1 }} data-testid="allergen-flags-editor">
      <Typography variant="subtitle2" gutterBottom>
        Allergen flags
      </Typography>
      <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1 }}>
        &quot;Contains&quot; is confirmed; &quot;may contain&quot; is advisory. No flags means no
        allergen information — not that it&apos;s safe.
      </Typography>
      {error && (
        <Alert severity="error" sx={{ mb: 1 }}>
          {error}
        </Alert>
      )}
      {local.map((f) => (
        <Box
          key={f.allergen.allergenID}
          sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}
        >
          <Typography variant="body2" sx={{ minWidth: 120 }}>
            {f.allergen.name}
          </Typography>
          <Select
            size="small"
            value={f.kind}
            disabled={pending}
            onChange={(e) => void apply(f.allergen.allergenID, e.target.value as AllergenFlagKind)}
            data-testid={`flag-kind-${f.allergen.allergenID}`}
          >
            <MenuItem value="contains">Contains</MenuItem>
            <MenuItem value="may_contain">May contain</MenuItem>
          </Select>
          <IconButton
            size="small"
            aria-label={`remove ${f.allergen.name}`}
            disabled={pending}
            onClick={() => void apply(f.allergen.allergenID, null)}
          >
            <DeleteIcon fontSize="small" />
          </IconButton>
        </Box>
      ))}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 1 }}>
        <Autocomplete<Allergen>
          size="small"
          sx={{ minWidth: 220 }}
          options={unflagged}
          getOptionLabel={(a) => a.name}
          isOptionEqualToValue={(o, v) => o.allergenID === v.allergenID}
          value={candidate}
          onChange={(_, v) => setCandidate(v)}
          renderInput={(params) => <TextField {...params} label="Add allergen" />}
        />
        <Button
          size="small"
          variant="outlined"
          disabled={!candidate || pending}
          onClick={() => {
            if (candidate) {
              void apply(candidate.allergenID, "contains");
              setCandidate(null);
            }
          }}
        >
          Add
        </Button>
      </Box>
    </Box>
  );
}
