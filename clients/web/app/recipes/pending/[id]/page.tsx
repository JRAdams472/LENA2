"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import ButtonGroup from "@mui/material/ButtonGroup";
import CircularProgress from "@mui/material/CircularProgress";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import DeleteIcon from "@mui/icons-material/Delete";
import AddIcon from "@mui/icons-material/Add";
import { api, ApiError } from "@/lib/api";
import { RecipeImport, RecipeImportReview, RecipeImportReviewItem, RecipeImportReviewStep } from "@/lib/types";
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

  const [review, setReview] = useState<RecipeImportReview | null>(null);

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
          <Stack spacing={2}>
            {review.items.map((item, i) => (
              <Stack key={i} direction="row" spacing={1} sx={{ alignItems: "center" }}>
                <TextField
                  label="Ingredient"
                  value={item.ingredient}
                  onChange={(e) => updateItem(i, { ingredient: e.target.value })}
                  size="small"
                  sx={{ flex: 2 }}
                />
                <TextField
                  label="Qty"
                  type="number"
                  value={item.quantity ?? ""}
                  onChange={(e) => updateItem(i, { quantity: e.target.value ? Number(e.target.value) : null })}
                  size="small"
                  sx={{ flex: 1 }}
                />
                <TextField
                  label="Unit"
                  value={item.unit ?? ""}
                  onChange={(e) => updateItem(i, { unit: e.target.value || null })}
                  size="small"
                  sx={{ flex: 1 }}
                />
                <TextField
                  label="Item ID"
                  value={item.itemId ?? ""}
                  onChange={(e) => updateItem(i, { itemId: e.target.value || null })}
                  size="small"
                  sx={{ flex: 1 }}
                />
                <TextField
                  label="Unit ID"
                  value={item.unitId ?? ""}
                  onChange={(e) => updateItem(i, { unitId: e.target.value || null })}
                  size="small"
                  sx={{ flex: 1 }}
                />
                <IconButton onClick={() => removeItem(i)} color="error" size="small" aria-label="Remove ingredient">
                  <DeleteIcon />
                </IconButton>
              </Stack>
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
          <Button onClick={() => approveMutation.mutate()} color="success">
            Approve
          </Button>
          <Button onClick={() => rejectMutation.mutate()} color="error">
            Reject
          </Button>
          <Button onClick={() => retryMutation.mutate()}>Retry</Button>
        </ButtonGroup>
      </Stack>
    </Box>
  );
}
