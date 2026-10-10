"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Autocomplete from "@mui/material/Autocomplete";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import Paper from "@mui/material/Paper";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TextField from "@mui/material/TextField";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import Typography from "@mui/material/Typography";
import { api } from "@/lib/api";
import { AllergenSuggestion, Recipe } from "@/lib/types";

type StatusFilter = "pending" | "accepted" | "dismissed" | "all";

const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: "pending", label: "Pending" },
  { value: "accepted", label: "Accepted" },
  { value: "dismissed", label: "Dismissed" },
  { value: "all", label: "All" },
];

const statusColor = (s: string) => {
  if (s === "accepted") return "success";
  if (s === "dismissed") return "default";
  return "warning";
};

// AI-proposed allergen flags held for human review. Nothing here writes a
// flag until Accept — which applies it under the reviewer's name — and the
// empty-flag rule still holds for unreviewed rows: no flag means "no
// allergen information", not "safe".
export default function AllergenSuggestionsPage() {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<StatusFilter>("pending");
  const [recipe, setRecipe] = useState<Recipe | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const aiQuery = useQuery({
    queryKey: ["aiAvailable"],
    queryFn: () => api.getAIAvailable(),
    staleTime: 5 * 60 * 1000,
  });

  const [recipeInput, setRecipeInput] = useState("");
  const [debouncedRecipeInput, setDebouncedRecipeInput] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebouncedRecipeInput(recipeInput), 300);
    return () => clearTimeout(t);
  }, [recipeInput]);

  const recipesQuery = useQuery({
    queryKey: ["recipe-autocomplete", debouncedRecipeInput],
    queryFn: () =>
      api.getRecipesPaged(1, 25, debouncedRecipeInput || undefined),
  });

  const queueQuery = useQuery({
    queryKey: ["allergen-suggestions", status],
    queryFn: () => api.getAllergenSuggestions(status === "all" ? undefined : status),
  });

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ["allergen-suggestions"] });
  };

  const suggestMutation = useMutation({
    mutationFn: (recipeId: number) => api.suggestRecipeAllergens(recipeId),
    onSuccess: (created) => {
      const plural = created.length === 1 ? "" : "s";
      setNotice(
        created.length === 0
          ? "No new proposals — every suggestion either duplicates an open row or a curated flag."
          : `${created.length} suggestion${plural} added to the queue.`
      );
      refresh();
    },
    onError: () => setNotice(null),
  });

  const acceptMutation = useMutation({
    mutationFn: (id: number) => api.acceptAllergenSuggestion(id),
    onSuccess: refresh,
  });

  const dismissMutation = useMutation({
    mutationFn: (id: number) => api.dismissAllergenSuggestion(id),
    onSuccess: refresh,
  });

  const rows = useMemo(() => queueQuery.data ?? [], [queueQuery.data]);
  const busy = suggestMutation.isPending;
  const actionErr =
    suggestMutation.error ?? acceptMutation.error ?? dismissMutation.error;

  const targetLabel = (r: AllergenSuggestion) =>
    r.targetKind === "item"
      ? (r.itemName ?? `item ${r.itemId}`)
      : (r.ingredientName ?? `ingredient ${r.ingredientId}`);

  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 1 }}>
        Allergen Flag Suggestions
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        The assistant proposes ingredient/item allergen flags from recipe
        contents. Proposals are never applied automatically — Accept writes
        the flag under your name; Dismiss keeps the audit trail.
      </Typography>

      {aiQuery.data === false && (
        <Alert severity="info" sx={{ mb: 2 }}>
          No AI provider is configured — the suggester is unavailable, but
          queued proposals can still be reviewed.
        </Alert>
      )}

      <Paper sx={{ p: 2, mb: 3 }}>
        <Typography variant="subtitle1" sx={{ mb: 1 }}>
          Generate suggestions for a recipe
        </Typography>
        <Box sx={{ display: "flex", gap: 2, alignItems: "center", flexWrap: "wrap" }}>
          <Autocomplete<Recipe>
            sx={{ minWidth: 320, flexGrow: 1, maxWidth: 480 }}
            size="small"
            options={
              recipe &&
              !(recipesQuery.data?.items ?? []).some(
                (o) => o.recipeID === recipe.recipeID
              )
                ? [recipe, ...(recipesQuery.data?.items ?? [])]
                : (recipesQuery.data?.items ?? [])
            }
            loading={recipesQuery.isLoading}
            getOptionLabel={(r) => r.recipeName}
            isOptionEqualToValue={(o, v) => o.recipeID === v.recipeID}
            filterOptions={(x) => x}
            inputValue={recipeInput}
            onInputChange={(_, v) => setRecipeInput(v)}
            value={recipe}
            onChange={(_, v) => setRecipe(v)}
            renderInput={(params) => <TextField {...params} label="Recipe" />}
          />
          <Button
            variant="contained"
            disabled={!recipe || busy || aiQuery.data === false}
            onClick={() => recipe && suggestMutation.mutate(recipe.recipeID)}
          >
            {busy ? <CircularProgress size={18} sx={{ mr: 1 }} /> : null}
            Suggest flags
          </Button>
        </Box>
        {notice && (
          <Alert severity="success" sx={{ mt: 2 }} onClose={() => setNotice(null)}>
            {notice}
          </Alert>
        )}
        {suggestMutation.error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {(suggestMutation.error as Error).message}
          </Alert>
        )}
      </Paper>

      <Box sx={{ display: "flex", alignItems: "center", gap: 2, mb: 2 }}>
        <Typography variant="subtitle1">Review queue</Typography>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={status}
          onChange={(_, v) => v && setStatus(v as StatusFilter)}
        >
          {STATUS_OPTIONS.map((o) => (
            <ToggleButton key={o.value} value={o.value}>
              {o.label}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
      </Box>

      {queueQuery.error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {(queueQuery.error as Error).message}
        </Alert>
      )}
      {actionErr && !suggestMutation.error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {(actionErr as Error).message}
        </Alert>
      )}

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Target</TableCell>
              <TableCell>Allergen</TableCell>
              <TableCell>Flag</TableCell>
              <TableCell>Recipe</TableCell>
              <TableCell>Rationale</TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {queueQuery.isLoading && (
              <TableRow>
                <TableCell colSpan={7} align="center">
                  <CircularProgress size={24} sx={{ my: 2 }} />
                </TableCell>
              </TableRow>
            )}
            {!queueQuery.isLoading && rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={7} align="center">
                  <Typography variant="body2" color="text.secondary" sx={{ my: 2 }}>
                    No {status === "all" ? "" : status + " "}suggestions.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {rows.map((r) => (
              <TableRow key={r.id}>
                <TableCell>
                  <Chip
                    size="small"
                    variant="outlined"
                    label={r.targetKind}
                    sx={{ mr: 1 }}
                  />
                  {targetLabel(r)}
                </TableCell>
                <TableCell>{r.allergen.name}</TableCell>
                <TableCell>
                  <Chip
                    size="small"
                    color={r.kind === "contains" ? "error" : "warning"}
                    variant="outlined"
                    label={r.kind === "contains" ? "contains" : "may contain"}
                  />
                </TableCell>
                <TableCell>{r.recipeName ?? "—"}</TableCell>
                <TableCell sx={{ maxWidth: 320 }}>
                  <Typography variant="body2" noWrap title={r.rationale ?? ""}>
                    {r.rationale ?? "—"}
                  </Typography>
                </TableCell>
                <TableCell>
                  <Chip
                    size="small"
                    color={statusColor(r.status)}
                    label={r.status}
                  />
                </TableCell>
                <TableCell align="right">
                  {r.status === "pending" && (
                    <Box sx={{ display: "flex", gap: 1, justifyContent: "flex-end" }}>
                      <Button
                        size="small"
                        variant="contained"
                        color="success"
                        disabled={acceptMutation.isPending || dismissMutation.isPending}
                        onClick={() => acceptMutation.mutate(r.id)}
                      >
                        Accept
                      </Button>
                      <Button
                        size="small"
                        variant="outlined"
                        disabled={acceptMutation.isPending || dismissMutation.isPending}
                        onClick={() => dismissMutation.mutate(r.id)}
                      >
                        Dismiss
                      </Button>
                    </Box>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  );
}
