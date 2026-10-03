"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Autocomplete from "@mui/material/Autocomplete";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import ButtonGroup from "@mui/material/ButtonGroup";
import Checkbox from "@mui/material/Checkbox";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import FormControlLabel from "@mui/material/FormControlLabel";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import DeleteIcon from "@mui/icons-material/Delete";
import AddIcon from "@mui/icons-material/Add";
import { api, ApiError } from "@/lib/api";
import { RecipeImport, RecipeImportReview, RecipeImportReviewItem, RecipeImportReviewStep, Unit } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";

function emptyReviewItem(): RecipeImportReviewItem {
  return {
    ingredient: "",
    quantity: null,
    unit: null,
    section: null,
    notes: null,
    isOptional: false,
    itemId: null,
    itemKind: null,
    itemName: null,
    unitId: null,
    confidence: 0,
    suggestions: [],
    status: "manual",
    approved: true,
  };
}

function emptyReviewStep(): RecipeImportReviewStep {
  return { stepNumber: 1, instruction: "" };
}

interface ItemOption {
  id: string;
  name: string;
  kind: "item" | "ingredient";
}

const statusColors: Record<string, "success" | "warning" | "error" | "default"> = {
  accepted: "success",
  suggested: "warning",
  unmatched: "error",
};

function IngredientRow({
  item,
  units,
  onChange,
  onRemove,
}: {
  item: RecipeImportReviewItem;
  units: Unit[];
  onChange: (patch: Partial<RecipeImportReviewItem>) => void;
  onRemove: () => void;
}) {
  const [itemInput, setItemInput] = useState("");
  const [debouncedItem, setDebouncedItem] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebouncedItem(itemInput), 300);
    return () => clearTimeout(t);
  }, [itemInput]);

  const itemSearch = useQuery({
    queryKey: ["item-search", debouncedItem],
    queryFn: () => api.searchItems(debouncedItem),
    enabled: debouncedItem.trim().length > 0,
  });

  const ingredientSearch = useQuery({
    queryKey: ["ingredient-search", debouncedItem],
    queryFn: () => api.searchIngredients(debouncedItem, 10),
    enabled: debouncedItem.trim().length > 0,
  });

  // The generic ingredient is the primary binding — ingredient options
  // lead the list; branded items follow.
  const itemOptions = useMemo(() => {
    const options: ItemOption[] = [];
    const seen = new Set<string>();
    const push = (o: ItemOption) => {
      const key = `${o.kind}-${o.id}`;
      if (!seen.has(key)) {
        seen.add(key);
        options.push(o);
      }
    };
    for (const s of item.suggestions) {
      if (s.kind === "ingredient") push({ id: s.id, name: s.name, kind: "ingredient" });
    }
    for (const g of ingredientSearch.data ?? []) {
      push({ id: String(g.ingredientID), name: g.name, kind: "ingredient" });
    }
    for (const s of item.suggestions) {
      if (s.kind === "item") push({ id: s.id, name: s.name, kind: "item" });
    }
    for (const it of itemSearch.data ?? []) {
      push({ id: String(it.itemID), name: it.name, kind: "item" });
    }
    return options;
  }, [item.suggestions, ingredientSearch.data, itemSearch.data]);

  // itemId carries whichever id was bound; itemKind tells the backend
  // which catalog namespace it belongs to ("ingredient" -> ingredient_id).
  const selectOption = (opt: ItemOption | null) => {
    onChange({
      itemId: opt?.id ?? null,
      itemKind: opt?.kind ?? null,
      itemName: opt?.name ?? null,
      approved: opt !== null,
    });
  };

  return (
    <Box>
      <Stack direction="row" spacing={1} sx={{ alignItems: "center" }}>
        <Chip
          size="small"
          label={item.status}
          color={statusColors[item.status] ?? "default"}
          sx={{ minWidth: 84 }}
        />
        <TextField
          label="Ingredient"
          value={item.ingredient}
          onChange={(e) => onChange({ ingredient: e.target.value })}
          size="small"
          sx={{ flex: 2 }}
        />
        <TextField
          label="Qty"
          type="number"
          value={item.quantity ?? ""}
          onChange={(e) => {
            const v = e.target.value;
            if (v === "") return onChange({ quantity: null });
            const n = Number(v);
            if (!Number.isNaN(n) && n >= 0) onChange({ quantity: n });
          }}
          slotProps={{ htmlInput: { min: 0, step: "any" } }}
          size="small"
          sx={{ width: 90 }}
        />
        <Autocomplete
          freeSolo
          size="small"
          options={units}
          getOptionLabel={(o) => (typeof o === "string" ? o : o.name)}
          isOptionEqualToValue={(o, v) => (typeof v === "string" ? o.name === v : o.unitID === v.unitID)}
          value={item.unit ?? ""}
          onChange={(_, v) => {
            const name = typeof v === "string" ? v : v?.name ?? "";
            const match = units.find((u) => u.name === name);
            onChange({ unit: name || null, unitId: match ? String(match.unitID) : null });
          }}
          renderInput={(params) => <TextField {...params} label="Unit" />}
          sx={{ width: 130 }}
        />
        <Autocomplete
          size="small"
          options={itemOptions}
          getOptionLabel={(o) =>
            o.name + (o.kind === "ingredient" ? " (ingredient)" : "")
          }
          isOptionEqualToValue={(a, b) => a.id === b.id && a.kind === b.kind}
          value={
            item.itemId
              ? {
                  id: item.itemId,
                  kind: (item.itemKind === "ingredient" ? "ingredient" : "item") as "item" | "ingredient",
                  name: item.itemName ?? item.itemId,
                }
              : null
          }
          inputValue={itemInput}
          onInputChange={(_, v) => setItemInput(v)}
          onChange={(_, v) => selectOption(v)}
          loading={itemSearch.isLoading || ingredientSearch.isLoading}
          noOptionsText="No ingredients or catalog items"
          renderInput={(params) => <TextField {...params} label="Catalog match" />}
          sx={{ flex: 2 }}
        />
        <FormControlLabel
          control={
            <Checkbox
              size="small"
              checked={item.approved}
              onChange={(e) => onChange({ approved: e.target.checked })}
            />
          }
          label="Resolved"
        />
        <IconButton onClick={onRemove} color="error" size="small" aria-label="Remove ingredient">
          <DeleteIcon />
        </IconButton>
      </Stack>
      {item.suggestions.length > 0 && (
        <Stack direction="row" spacing={0.5} sx={{ pl: 12, mt: 0.5, flexWrap: "wrap" }}>
          {[...item.suggestions]
            .sort((a, b) =>
              a.kind === b.kind ? b.score - a.score : a.kind === "ingredient" ? -1 : 1
            )
            .map((s) => (
              <Chip
                key={`${s.kind}-${s.id}`}
                size="small"
                variant="outlined"
                label={`${s.name}${s.kind === "ingredient" ? " (ingredient)" : ""} ${Math.round(s.score * 100)}%`}
                onClick={() =>
                  selectOption({ id: s.id, name: s.name, kind: s.kind === "ingredient" ? "ingredient" : "item" })
                }
              />
            ))}
        </Stack>
      )}
    </Box>
  );
}

function reviewFromImport(ri: RecipeImport): RecipeImportReview {
  return (
    ri.review ?? {
      pageId: null,
      name: ri.draft?.name ?? null,
      description: ri.draft?.description ?? null,
      servings: ri.draft?.servings ?? null,
      prepTimeMinutes: ri.draft?.prepTimeMinutes ?? null,
      cookTimeMinutes: ri.draft?.cookTimeMinutes ?? null,
      sourceHint: ri.draft?.sourceHint ?? null,
      items: (ri.draft?.items ?? []).map((d) => ({ ...emptyReviewItem(), ...d, confidence: 1, suggestions: [], status: "manual", approved: true })),
      steps: ri.draft?.steps ?? [],
      approved: false,
    }
  );
}

export default function PendingRecipeDetailPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params?.id);
  const router = useRouter();
  const { isAdmin, isLoading: meLoading } = useMe();
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);

  const { data: recipeImport, isLoading } = useQuery({
    queryKey: ["recipe-import", id],
    queryFn: () => api.getRecipeImport(id),
    enabled: isAdmin && !Number.isNaN(id),
    refetchInterval: 5000,
  });

  const { data: units = [] } = useQuery({
    queryKey: ["units"],
    queryFn: () => api.getUnits(),
    enabled: isAdmin,
  });

  const [review, setReview] = useState<RecipeImportReview | null>(null);
  const [zeroQtyIndex, setZeroQtyIndex] = useState<number | null>(null);
  const [confirmReject, setConfirmReject] = useState(false);

  if (recipeImport && !review) {
    setReview(reviewFromImport(recipeImport));
  }

  const updateMutation = useMutation({
    mutationFn: (r: RecipeImportReview) => api.updateRecipeImport(id, r),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["recipe-import", id] });
      queryClient.invalidateQueries({ queryKey: ["pending-recipe-imports"] });
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : "Failed to update review"),
  });

  const approveMutation = useMutation({
    mutationFn: () => api.approveRecipeImport(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["recipe-import", id] });
      queryClient.invalidateQueries({ queryKey: ["pending-recipe-imports"] });
      router.push("/recipes/pending");
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : "Failed to approve"),
  });

  const rejectMutation = useMutation({
    mutationFn: () => api.rejectRecipeImport(id),
    onSuccess: () => router.push("/recipes/pending"),
    onError: (e) => setError(e instanceof ApiError ? e.message : "Failed to reject"),
  });

  const retryMutation = useMutation({
    mutationFn: () => api.retryRecipeImport(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["recipe-import", id] });
      queryClient.invalidateQueries({ queryKey: ["pending-recipe-imports"] });
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : "Failed to retry"),
  });

  if (meLoading || isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!isAdmin) {
    return <Alert severity="error">Forbidden: this page requires the admin role.</Alert>;
  }

  if (!recipeImport || !review) {
    return <Alert severity="error">Recipe import not found.</Alert>;
  }

  const updateField = <K extends keyof RecipeImportReview>(field: K, value: RecipeImportReview[K]) => {
    setReview((r) => (r ? { ...r, [field]: value } : r));
  };

  const updateItem = (index: number, patch: Partial<RecipeImportReviewItem>) => {
    // A zero quantity means the ingredient is gone — ask before removing it.
    if (patch.quantity === 0) {
      setZeroQtyIndex(index);
      return;
    }
    setReview((r) => {
      if (!r) return r;
      const items = [...r.items];
      items[index] = { ...items[index], ...patch };
      return { ...r, items };
    });
  };

  const removeItem = (index: number) => {
    setReview((r) => (r ? { ...r, items: r.items.filter((_, i) => i !== index) } : r));
  };

  const addItem = () => {
    setReview((r) => (r ? { ...r, items: [...r.items, emptyReviewItem()] } : r));
  };

  const updateStep = (index: number, instruction: string) => {
    setReview((r) => {
      if (!r) return r;
      const steps = [...r.steps];
      steps[index] = { ...steps[index], instruction };
      return { ...r, steps };
    });
  };

  const removeStep = (index: number) => {
    setReview((r) => (r ? { ...r, steps: r.steps.filter((_, i) => i !== index) } : r));
  };

  const addStep = () => {
    setReview((r) => (r ? { ...r, steps: [...r.steps, { ...emptyReviewStep(), stepNumber: r.steps.length + 1 }] } : r));
  };

  const anyLoading = updateMutation.isPending || approveMutation.isPending || rejectMutation.isPending || retryMutation.isPending;

  return (
    <Box>
      <Typography variant="h5" gutterBottom>
        Review Import: {recipeImport.sourceFilename}
      </Typography>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}
      <Stack spacing={2}>
        <Paper sx={{ p: 2 }}>
          <Stack spacing={2}>
            <TextField
              label="Recipe Name"
              value={review.name ?? ""}
              onChange={(e) => updateField("name", e.target.value || null)}
              fullWidth
            />
            <TextField
              label="Description"
              value={review.description ?? ""}
              onChange={(e) => updateField("description", e.target.value || null)}
              multiline
              rows={2}
              fullWidth
            />
            <Stack direction="row" spacing={2}>
              <TextField
                label="Servings"
                type="number"
                value={review.servings ?? ""}
                onChange={(e) => updateField("servings", e.target.value ? Number(e.target.value) : null)}
              />
              <TextField
                label="Prep Time (min)"
                type="number"
                value={review.prepTimeMinutes ?? ""}
                onChange={(e) => updateField("prepTimeMinutes", e.target.value ? Number(e.target.value) : null)}
              />
              <TextField
                label="Cook Time (min)"
                type="number"
                value={review.cookTimeMinutes ?? ""}
                onChange={(e) => updateField("cookTimeMinutes", e.target.value ? Number(e.target.value) : null)}
              />
            </Stack>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography variant="h6" gutterBottom>
            Ingredients
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: -1 }}>
            Each row needs a catalog item and the Resolved check before the recipe can be approved.
            Unit may be left blank — it defaults to &quot;each&quot; (pick &quot;to taste&quot; for unquantified seasoning).
          </Typography>
          <Stack spacing={2}>
            {review.items.map((item, i) => (
              <IngredientRow
                key={i}
                item={item}
                units={units}
                onChange={(patch) => updateItem(i, patch)}
                onRemove={() => removeItem(i)}
              />
            ))}
            <Button startIcon={<AddIcon />} onClick={addItem} size="small">
              Add Ingredient
            </Button>
          </Stack>
        </Paper>

        <Paper sx={{ p: 2 }}>
          <Typography variant="h6" gutterBottom>
            Steps
          </Typography>
          <Stack spacing={2}>
            {review.steps.map((step, i) => (
              <Stack key={i} direction="row" spacing={1} sx={{ alignItems: "center" }}>
                <Typography sx={{ minWidth: 24 }}>{i + 1}.</Typography>
                <TextField
                  value={step.instruction}
                  onChange={(e) => updateStep(i, e.target.value)}
                  size="small"
                  fullWidth
                />
                <IconButton onClick={() => removeStep(i)} color="error" size="small" aria-label="Remove step">
                  <DeleteIcon />
                </IconButton>
              </Stack>
            ))}
            <Button startIcon={<AddIcon />} onClick={addStep} size="small">
              Add Step
            </Button>
          </Stack>
        </Paper>

        <ButtonGroup variant="contained" disabled={anyLoading}>
          <Button onClick={() => updateMutation.mutate(review)}>Save Review</Button>
          <Button
            onClick={() => updateMutation.mutate(review, { onSuccess: () => approveMutation.mutate() })}
            color="success"
          >
            Approve
          </Button>
          <Button onClick={() => setConfirmReject(true)} color="error">
            Reject
          </Button>
          <Button
            onClick={() => retryMutation.mutate()}
            disabled={anyLoading || (recipeImport.status !== "failed" && recipeImport.status !== "profanity")}
          >
            Retry
          </Button>
        </ButtonGroup>
      </Stack>

      <Dialog open={zeroQtyIndex !== null} onClose={() => {
        if (zeroQtyIndex !== null) updateItem(zeroQtyIndex, { quantity: 1 });
        setZeroQtyIndex(null);
      }}>
        <DialogTitle>Remove this ingredient?</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            A quantity of 0 doesn&apos;t make sense in a recipe. Remove
            {zeroQtyIndex !== null && review?.items[zeroQtyIndex]
              ? ` "${review.items[zeroQtyIndex].ingredient}"`
              : " this ingredient"}{" "}
            instead, or keep it with a quantity of 1?
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              if (zeroQtyIndex !== null) updateItem(zeroQtyIndex, { quantity: 1 });
              setZeroQtyIndex(null);
            }}
          >
            No, keep at 1
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => {
              if (zeroQtyIndex !== null) removeItem(zeroQtyIndex);
              setZeroQtyIndex(null);
            }}
          >
            Yes, remove it
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={confirmReject} onClose={() => setConfirmReject(false)}>
        <DialogTitle>Reject this import?</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Rejecting permanently discards {recipeImport.sourceFilename} — it
            cannot be retried or approved afterward.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmReject(false)}>Cancel</Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => {
              setConfirmReject(false);
              rejectMutation.mutate();
            }}
          >
            Yes, reject it
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
