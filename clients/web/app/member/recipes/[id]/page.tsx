"use client";

import { Fragment, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
import IconButton from "@mui/material/IconButton";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Radio from "@mui/material/Radio";
import RadioGroup from "@mui/material/RadioGroup";
import Select from "@mui/material/Select";
import Divider from "@mui/material/Divider";
import Paper from "@mui/material/Paper";
import Autocomplete from "@mui/material/Autocomplete";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import Rating from "@mui/material/Rating";
import TableRow from "@mui/material/TableRow";
import TextField from "@mui/material/TextField";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import Typography from "@mui/material/Typography";
import CloseIcon from "@mui/icons-material/Close";
import * as aiSuggest from "@/lib/ai/suggest";
import { useLocalEngineReady } from "@/lib/ai/engineStore";
import { AllergyWarningsAlert, AllergenFlagsLine } from "@/app/components/AllergyWarning";
import { api } from "@/lib/api";
import { Ingredient, Item, PairingSuggestion, Recipe, RecipeItem, RecipeStep, RecipeView } from "@/lib/types";
import IngredientAutocomplete from "@/app/components/IngredientAutocomplete";
import { brandedName, brandSuffix, fmtQty, recipeItemLabel } from "@/lib/format";
import {
  DeltaDraft,
  deltaBadgeLabel,
  draftForLine,
  draftForStep,
  draftFromDelta,
  draftIsDirty,
  upsertItemDraft,
  upsertStepDraft,
} from "@/lib/delta";
import {
  DeltaBadge,
  ItemTweakEditor,
  RecipeDeltaPanel,
  StepTweakEditor,
} from "@/app/member/recipes/[id]/recipe-delta";
import { isOfDrinkingAge } from "@/lib/age";
import AlcoholGateHint from "../AlcoholGateHint";
import DisclaimerLine from "@/app/components/DisclaimerLine";
import { useMe } from "@/app/auth/useMe";

function ratingSummary(average: number | null, count: number): string {
  if (average == null) return "No ratings yet";
  const plural = count === 1 ? "" : "s";
  return `${average.toFixed(1)} avg · ${count} rating${plural}`;
}

function brandItemLabel(item: { name: string; brand?: string | null }): string {
  return brandedName(item.brand, item.name);
}



export default function RecipeDetailPage() {
  const params = useParams<{ id: string }>();
  const recipeId = Number(params.id);
  const queryClient = useQueryClient();

  const [selectedIngredient, setSelectedIngredient] = useState<Ingredient | null>(null);
  const [selectedItem, setSelectedItem] = useState<Item | null>(null);
  const [portion, setPortion] = useState("");
  const [unit, setUnit] = useState("");
  const [isOptional, setIsOptional] = useState(false);

  // Where the new/edited step lands: "beginning", "end", or
  // `after-<recipeStepID>` — the raw step number is derived, never typed.
  const [stepPosition, setStepPosition] = useState("end");
  const [instruction, setInstruction] = useState("");
  const [stepDuration, setStepDuration] = useState("");
  const [stepType, setStepType] = useState("");
  const [stepPassive, setStepPassive] = useState(false);
  const [stepDependsOn, setStepDependsOn] = useState("");
  const [stepAppliance, setStepAppliance] = useState("");
  const [editingStepId, setEditingStepId] = useState<number | null>(null);
  const [debouncedIngredientInput, setDebouncedIngredientInput] = useState("");
  const [itemSearch, setItemSearch] = useState("");
  const [debouncedItemSearch, setDebouncedItemSearch] = useState("");

  // Household delta UI state: the recipe view toggle, the editable change
  // set, the stale-banner dismissal, and which line/step editor is open.
  const [view, setView] = useState<RecipeView>("effective");
  const [draft, setDraft] = useState<DeltaDraft>({ items: [], steps: [] });
  const [staleDismissed, setStaleDismissed] = useState(false);
  const [tweakItemId, setTweakItemId] = useState<number | null>(null);
  const [tweakStepId, setTweakStepId] = useState<number | null>(null);
  const draftVersionRef = useRef("");

  const { me, isAdmin } = useMe();

  const localAIReady = useLocalEngineReady();
  const aiQuery = useQuery({
    queryKey: ["aiAvailable"],
    queryFn: () => api.getAIAvailable(),
    staleTime: 5 * 60 * 1000,
  });

  const [pairings, setPairings] = useState<PairingSuggestion[] | null>(null);
  const pairingMutation = useMutation({
    mutationFn: () => aiSuggest.suggestPairings(recipeId),
    onSuccess: (data) => setPairings(data),
  });
  const aiAvailable = aiQuery.data === true || localAIReady;
  const canPair = aiAvailable && isOfDrinkingAge(me?.birthdate);

  const recipeQuery = useQuery({
    queryKey: ["recipe", recipeId, view],
    queryFn: () => api.getRecipe(recipeId, view),
    enabled: !Number.isNaN(recipeId),
  });

  // Canonical rows for the tweaks panel's "Swap X for Y" descriptions —
  // the effective view's labels are already changed.
  const canonicalQuery = useQuery({
    queryKey: ["recipe-canonical", recipeId],
    queryFn: () => api.getRecipe(recipeId, "canonical"),
    enabled: !Number.isNaN(recipeId),
  });

  const groupsQuery = useQuery({
    queryKey: ["recipe-category-groups"],
    queryFn: api.getRecipeCategoryGroups,
    staleTime: 60_000,
    enabled: isAdmin,
  });

  useEffect(() => {
    if (!Number.isNaN(recipeId)) void api.recordView("recipe", recipeId);
  }, [recipeId]);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedItemSearch(itemSearch), 300);
    return () => clearTimeout(timer);
  }, [itemSearch]);

  useEffect(() => {
    if (debouncedItemSearch.trim()) {
      void api.recordSearch("item", debouncedItemSearch);
    }
  }, [debouncedItemSearch]);

  useEffect(() => {
    if (debouncedIngredientInput.trim()) {
      void api.recordSearch("ingredient", debouncedIngredientInput);
    }
  }, [debouncedIngredientInput]);

  const searchQuery = useQuery({
    queryKey: ["items-search", debouncedItemSearch],
    queryFn: () => api.searchItems(debouncedItemSearch, undefined, 50),
    enabled: debouncedItemSearch.length >= 2,
  });

  const recipeItemsQuery = useQuery({
    queryKey: ["recipe-items", recipeId, view],
    queryFn: () => api.getRecipeItems(recipeId, view),
    enabled: !Number.isNaN(recipeId),
  });

  const recipeStepsQuery = useQuery({
    queryKey: ["recipe-steps", recipeId, view],
    queryFn: () => api.getRecipeSteps(recipeId, view),
    enabled: !Number.isNaN(recipeId),
  });

  const delta = recipeQuery.data?.householdDelta ?? null;

  // Seed the editor draft from the saved delta — but only when the saved
  // change set actually moves, so unrelated refetches (rating, categories)
  // don't wipe unsaved tweaks.
  useEffect(() => {
    const version = delta ? `${delta.recipeDeltaID}:${delta.updatedAt}` : "none";
    if (draftVersionRef.current !== version) {
      draftVersionRef.current = version;
      setDraft(draftFromDelta(delta));
    }
  }, [delta]);

  // Unsaved-tweaks guard: browsers get the native leave prompt via
  // beforeunload. In-app client-side navigation (next/link) can't be
  // intercepted in the App Router — a known limit; the "Unsaved changes"
  // caption in the panel is the in-app signal.
  const deltaDirty = draftIsDirty(draft, delta);
  useEffect(() => {
    if (!deltaDirty) return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [deltaDirty]);

  const acknowledgeMutation = useMutation({
    mutationFn: () => api.acknowledgeRecipeDelta(recipeId),
    onSuccess: () => {
      setStaleDismissed(false);
      return queryClient.invalidateQueries({ queryKey: ["recipe", recipeId] });
    },
  });

  const unitsQuery = useQuery({
    queryKey: ["units"],
    queryFn: api.getUnits,
    staleTime: 60_000,
  });

  const invalidateItems = () =>
    queryClient.invalidateQueries({ queryKey: ["recipe-items", recipeId] });
  const invalidateSteps = () =>
    queryClient.invalidateQueries({ queryKey: ["recipe-steps", recipeId] });

  const addItemMutation = useMutation({
    mutationFn: (payload: {
      itemId?: number | null;
      ingredientId?: number | null;
      portion: number;
      unit: string | null;
      isOptional: boolean;
    }) => api.addRecipeItem(recipeId, payload),
    onSuccess: () => {
      setSelectedIngredient(null);
      setDebouncedIngredientInput("");
      setSelectedItem(null);
      setItemSearch("");
      setDebouncedItemSearch("");
      setPortion("");
      setUnit("");
      setIsOptional(false);
      return invalidateItems();
    },
  });

  const removeItemMutation = useMutation({
    mutationFn: (item: RecipeItem) => api.removeRecipeItem(recipeId, item),
    onSuccess: invalidateItems,
  });

  // Step writes go through one whole-list replace (updateRecipe with
  // replaceContents rewrites recipe_step rows atomically), so inserting
  // mid-list just means computing the new ordering + numbers here.
  const saveStepsMutation = useMutation({
    mutationFn: (steps: RecipeStep[]) =>
      api.updateRecipe(recipeId, { recipeSteps: steps }, { replaceContents: true }),
    onSuccess: () => {
      resetStepForm();
      return invalidateSteps();
    },
  });

  const deleteStepMutation = useMutation({
    mutationFn: (stepId: number) => api.deleteRecipeStep(recipeId, stepId),
    onSuccess: invalidateSteps,
  });

  // recipeQuery is view-keyed — merge the fields each mutation owns into
  // the cached recipe instead of writing the mutation response wholesale
  // (it carries no householdDelta and effective-only items).
  const rateMutation = useMutation({
    mutationFn: (rating: number) => api.rateRecipe(recipeId, rating),
    onSuccess: (updated) => {
      queryClient.setQueryData(["recipe", recipeId, view], (old: Recipe | undefined) =>
        old
          ? {
              ...old,
              myRating: updated.myRating,
              averageRating: updated.averageRating,
              ratingCount: updated.ratingCount,
            }
          : old
      );
      queryClient.invalidateQueries({ queryKey: ["recipe", recipeId] });
    },
  });

  // Rating's clear gesture (re-clicking the active star → null) un-rates.
  const clearRatingMutation = useMutation({
    mutationFn: () => api.clearRecipeRating(recipeId),
    onSuccess: () => {
      queryClient.setQueryData(["recipe", recipeId, view], (old: Recipe | undefined) =>
        old ? { ...old, myRating: null } : old
      );
      queryClient.invalidateQueries({ queryKey: ["recipe", recipeId] });
    },
  });

  const setCategoriesMutation = useMutation({
    mutationFn: (categoryIds: number[]) =>
      api.setRecipeCategories(recipeId, categoryIds),
    onSuccess: (updated) => {
      queryClient.setQueryData(["recipe", recipeId, view], (old: Recipe | undefined) =>
        old ? { ...old, categories: updated.categories } : old
      );
      queryClient.invalidateQueries({ queryKey: ["recipe", recipeId] });
      queryClient.invalidateQueries({ queryKey: ["recipes"] });
    },
  });

  const resetStepForm = () => {
    setEditingStepId(null);
    setStepPosition("end");
    setInstruction("");
    setStepDuration("");
    setStepType("");
    setStepPassive(false);
    setStepDependsOn("");
    setStepAppliance("");
  };

  const stepTimingFields = () => ({
    durationMinutes: stepDuration === "" ? null : Number(stepDuration),
    stepType: stepType === "" ? null : stepType,
    isPassive: stepPassive,
    dependsOnStepNumber: stepDependsOn === "" ? null : Number(stepDependsOn),
    appliance: stepAppliance === "" ? null : stepAppliance,
  });

  const handleAddItem = () => {
    if ((!selectedIngredient && !selectedItem) || portion === "") return;
    addItemMutation.mutate({
      ingredientId: selectedIngredient?.ingredientID ?? null,
      itemId: selectedItem?.itemID ?? null,
      portion: Number(portion),
      unit: unit === "" ? null : unit,
      isOptional,
    });
  };

  // Step writes happen on the CANONICAL list — the displayed steps can be
  // the delta-applied effective view, and writing those rows back would
  // bake household tweaks into the shared recipe.
  const canonicalSteps = [...(canonicalQuery.data?.recipeSteps ?? [])].sort(
    (a, b) => a.stepNumber - b.stepNumber
  );

  const handleSaveStep = () => {
    if (instruction.trim() === "") return;
    const editingStep =
      editingStepId === null
        ? null
        : canonicalSteps.find((s) => s.recipeStepID === editingStepId) ?? null;
    // Insert position is counted among the OTHER steps (the edited step
    // is lifted out and re-inserted at the chosen spot).
    const others = canonicalSteps.filter(
      (s) => s.recipeStepID !== editingStepId
    );
    let pos =
      stepPosition === "beginning"
        ? 0
        : stepPosition === "end"
          ? others.length
          : others.findIndex(
                (s) => `after-${s.recipeStepID}` === stepPosition
              ) + 1;
    if (pos < 0 || pos > others.length) pos = others.length; // stale pick → append

    // Step numbers must stay UNIQUE (recipe_id, step_number). Prefer a
    // free number in the gap; when the slot sits inside a contiguous run
    // the whole suffix shifts up by one and depends_on references get
    // remapped to match.
    const prev = pos > 0 ? others[pos - 1].stepNumber : 0;
    const next = pos < others.length ? others[pos].stepNumber : null;
    const shifted = new Set<number>();
    let newNumber: number;
    if (next === null || next - prev > 1) {
      newNumber = prev + 1;
    } else {
      newNumber = next;
      for (const s of others.slice(pos)) shifted.add(s.stepNumber);
    }
    const bump = (n: number | null | undefined) =>
      n != null && shifted.has(n) ? n + 1 : (n ?? null);
    const renumbered: RecipeStep[] = others.map((s) => ({
      ...s,
      stepNumber: shifted.has(s.stepNumber) ? s.stepNumber + 1 : s.stepNumber,
      dependsOnStepNumber: bump(s.dependsOnStepNumber),
    }));

    const dep =
      stepDependsOn === "" ? null : bump(Number(stepDependsOn));
    const step: RecipeStep = editingStep
      ? {
          ...editingStep,
          stepNumber: newNumber,
          instruction,
          ...stepTimingFields(),
          dependsOnStepNumber: dep,
        }
      : {
          createdBy: "",
          createDate: "",
          lastUpdatedBy: null,
          lastUpdatedDate: null,
          recipeStepID: 0,
          recipeID: recipeId,
          stepNumber: newNumber,
          instruction,
          ...stepTimingFields(),
          dependsOnStepNumber: dep,
        };
    renumbered.splice(pos, 0, step);
    saveStepsMutation.mutate(renumbered);
  };

  const handleEditStep = (step: RecipeStep) => {
    setEditingStepId(step.recipeStepID);
    // Reflect the step's current spot in the position select.
    const idx = canonicalSteps.findIndex(
      (s) => s.recipeStepID === step.recipeStepID
    );
    setStepPosition(
      idx <= 0
        ? "beginning"
        : `after-${canonicalSteps[idx - 1].recipeStepID}`
    );
    setInstruction(step.instruction);
    setStepDuration(step.durationMinutes != null ? String(step.durationMinutes) : "");
    setStepType(step.stepType ?? "");
    setStepPassive(step.isPassive ?? false);
    setStepDependsOn(step.dependsOnStepNumber != null ? String(step.dependsOnStepNumber) : "");
    setStepAppliance(step.appliance ?? "");
  };

  const handleDeleteStep = (step: RecipeStep) => {
    if (window.confirm("Delete this step?")) {
      deleteStepMutation.mutate(step.recipeStepID);
    }
  };

  const sortedSteps = [...(recipeStepsQuery.data ?? [])].sort(
    (a, b) => a.stepNumber - b.stepNumber
  );

  if (Number.isNaN(recipeId)) {
    return <Alert severity="error">Invalid recipe id</Alert>;
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Paper sx={{ p: 3 }}>
        {recipeQuery.isLoading && <CircularProgress />}
        {recipeQuery.error && (
          <Alert severity="error">
            {(recipeQuery.error as Error).message}
          </Alert>
        )}
        {recipeQuery.data && (
          <>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
              <Typography variant="h4" gutterBottom>
                {recipeQuery.data.recipeName}
              </Typography>
              {delta && (
                <Chip
                  size="small"
                  color="primary"
                  variant="outlined"
                  label="Household version"
                  sx={{ mb: 1 }}
                />
              )}
            </Box>
            <Typography color="text.secondary" gutterBottom>
              {recipeQuery.data.description ?? "No description"}
            </Typography>
            {(delta || isAdmin) && (
              <ToggleButtonGroup
                size="small"
                exclusive
                value={view}
                onChange={(_, v) => {
                  if (v) setView(v);
                }}
                sx={{ mt: 0.5, mb: 1 }}
                aria-label="Recipe version"
              >
                <ToggleButton value="effective">Household version</ToggleButton>
                <ToggleButton value="canonical">Original recipe</ToggleButton>
              </ToggleButtonGroup>
            )}
            {view === "canonical" && delta && (
              <Alert severity="info" sx={{ mt: 1 }}>
                Showing the original recipe — your household&apos;s tweaks are
                hidden.
              </Alert>
            )}
            {delta?.stale && !staleDismissed && (
              <Alert
                severity="warning"
                sx={{ mt: 1 }}
                action={
                  <Box sx={{ display: "flex", alignItems: "center" }}>
                    <Button
                      size="small"
                      color="inherit"
                      onClick={() => acknowledgeMutation.mutate()}
                      disabled={acknowledgeMutation.isPending}
                    >
                      Mark reviewed
                    </Button>
                    <IconButton
                      size="small"
                      aria-label="Dismiss"
                      color="inherit"
                      onClick={() => setStaleDismissed(true)}
                    >
                      <CloseIcon fontSize="small" />
                    </IconButton>
                  </Box>
                }
              >
                The original recipe changed after these tweaks were saved.
                {delta.orphanedItemCount + delta.orphanedStepCount > 0 &&
                  ` ${delta.orphanedItemCount + delta.orphanedStepCount} tweak(s) no longer apply.`}
              </Alert>
            )}
            {acknowledgeMutation.error && (
              <Alert severity="error" sx={{ mt: 1 }}>
                {(acknowledgeMutation.error as Error).message}
              </Alert>
            )}
            <Typography variant="body2">
              Servings: {recipeQuery.data.servings ?? "-"}
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 1 }}>
              <Rating
                aria-label="Your rating"
                value={recipeQuery.data.myRating}
                onChange={(_, value) => {
                  if (value != null) rateMutation.mutate(value);
                  else if (recipeQuery.data.myRating != null)
                    clearRatingMutation.mutate();
                }}
              />
              <Typography variant="body2" color="text.secondary">
                {ratingSummary(
                  recipeQuery.data.averageRating,
                  recipeQuery.data.ratingCount
                )}
              </Typography>
              {recipeQuery.data.myRating != null && (
                <Button
                  size="small"
                  onClick={() => clearRatingMutation.mutate()}
                  disabled={clearRatingMutation.isPending}
                >
                  Clear
                </Button>
              )}
            </Box>
            {(recipeQuery.data.categories ?? []).length > 0 && (
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, mt: 1 }}>
                {(recipeQuery.data.categories ?? []).map((c) => (
                  <Chip
                    key={c.categoryID}
                    size="small"
                    variant="outlined"
                    label={`${c.group.groupName}: ${c.categoryName}`}
                  />
                ))}
              </Box>
            )}
            <AllergyWarningsAlert
              warnings={recipeQuery.data.allergyWarnings}
              sx={{ mt: 1 }}
            />
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, mt: 1, flexWrap: "wrap" }}>
              <Typography variant="caption" color="text.secondary">
                Allergens:
              </Typography>
              <AllergenFlagsLine allergens={recipeQuery.data.allergens} />
            </Box>
            <Box sx={{ mt: 1 }}>
              <DisclaimerLine />
            </Box>
            {rateMutation.error && (
              <Alert severity="error" sx={{ mt: 1 }}>
                {(rateMutation.error as Error).message}
              </Alert>
            )}
            {aiAvailable && !isOfDrinkingAge(me?.birthdate) && (
              <Box sx={{ mt: 2 }}>
                <AlcoholGateHint />
              </Box>
            )}
            {canPair && (
              <Box sx={{ mt: 2 }}>
                <Button
                  variant="outlined"
                  size="small"
                  onClick={() => pairingMutation.mutate()}
                  disabled={pairingMutation.isPending}
                >
                  {pairingMutation.isPending ? "Thinking…" : "Suggest wine pairing"}
                </Button>
                {pairingMutation.error && (
                  <Alert severity="error" sx={{ mt: 1 }}>
                    {(pairingMutation.error as Error).message}
                  </Alert>
                )}
                {pairings !== null && pairings.length === 0 && (
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                    No pairing suggestions for this dish.
                  </Typography>
                )}
                {pairings?.map((p) => (
                  <Box
                    key={`${p.bottleId ?? "style"}-${p.name}`}
                    sx={{ display: "flex", alignItems: "center", gap: 1, mt: 1 }}
                  >
                    <Typography variant="body2">
                      {p.name} — {p.reason}
                    </Typography>
                    {p.inCellar && (
                      <Chip size="small" color="success" label="In your cellar" />
                    )}
                  </Box>
                ))}
              </Box>
            )}
          </>
        )}
      </Paper>

      {isAdmin && (
      <Paper sx={{ p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Categories
        </Typography>
        {groupsQuery.isLoading && <CircularProgress size={20} />}
        {setCategoriesMutation.error && (
          <Alert severity="error" sx={{ mb: 1 }}>
            {(setCategoriesMutation.error as Error).message}
          </Alert>
        )}
        {recipeQuery.data && (groupsQuery.data ?? []).length > 0 && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {(groupsQuery.data ?? []).map((g) => {
              const selected = new Set(
                (recipeQuery.data.categories ?? [])
                  .filter((c) => c.group.categoryGroupID === g.categoryGroupID)
                  .map((c) => c.categoryID)
              );
              const current = () =>
                (recipeQuery.data.categories ?? []).map((c) => c.categoryID);
              const pick = (id: number) => {
                if (g.exclusive) {
                  const kept = current().filter(
                    (cid) => !g.categories.some((c) => c.categoryID === cid)
                  );
                  setCategoriesMutation.mutate(selected.has(id) ? kept : [...kept, id]);
                } else {
                  setCategoriesMutation.mutate(
                    selected.has(id)
                      ? current().filter((cid) => cid !== id)
                      : [...current(), id]
                  );
                }
              };
              return (
                <Box key={g.categoryGroupID}>
                  <Typography variant="subtitle2" color="text.secondary">
                    {g.groupName}
                    {g.exclusive ? " (pick one)" : ""}
                  </Typography>
                  {g.exclusive ? (
                    <RadioGroup
                      row
                      value={g.categories.find((c) => selected.has(c.categoryID))?.categoryID ?? ""}
                      onChange={(e) => pick(Number(e.target.value))}
                    >
                      {g.categories.map((c) => (
                        <FormControlLabel
                          key={c.categoryID}
                          value={c.categoryID}
                          control={<Radio size="small" />}
                          label={c.categoryName}
                        />
                      ))}
                    </RadioGroup>
                  ) : (
                    <Box sx={{ display: "flex", flexWrap: "wrap" }}>
                      {g.categories.map((c) => (
                        <FormControlLabel
                          key={c.categoryID}
                          control={
                            <Checkbox
                              size="small"
                              checked={selected.has(c.categoryID)}
                              onChange={() => pick(c.categoryID)}
                            />
                          }
                          label={c.categoryName}
                        />
                      ))}
                    </Box>
                  )}
                </Box>
              );
            })}
          </Box>
        )}
      </Paper>
      )}

      <Paper sx={{ p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Ingredients
        </Typography>

        {isAdmin && (
        <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", mb: 2 }}>
          <IngredientAutocomplete
            value={selectedIngredient}
            onChange={setSelectedIngredient}
            onSelect={(ing) => void api.recordSelection("ingredient", ing.ingredientID)}
            onInputTextChange={setDebouncedIngredientInput}
            sx={{ minWidth: 240 }}
          />
          <Autocomplete
            size="small"
            options={searchQuery.data ?? []}
            getOptionLabel={(item) => item?.name ?? ""}
            isOptionEqualToValue={(option, value) =>
              option?.itemID === value?.itemID
            }
            inputValue={itemSearch}
            onInputChange={(_, value) => setItemSearch(value)}
            value={selectedItem}
            onChange={(_, value) => {
              setSelectedItem(value);
              // A picked brand auto-fills the ingredient when the item is
              // linked — the line binds both keys.
              if (value && !selectedIngredient) {
                const linked = value.householdIngredient ?? value.ingredient;
                if (linked) setSelectedIngredient(linked);
              }
              if (value) void api.recordSelection("item", value.itemID);
            }}
            filterOptions={(options) => options}
            loading={searchQuery.isLoading}
            noOptionsText={
              debouncedItemSearch.length < 2
                ? "Type at least 2 characters"
                : "No items found"
            }
            renderOption={(props, item) => {
              const { key, ...liProps } = props;
              return (
                <li key={item.itemID} {...liProps}>
                  {item.name}
                  {brandSuffix(item.brand, item.name)}
                </li>
              );
            }}
            renderInput={(params) => (
              <TextField
                {...params}
                label="Preferred brand (optional)"
                size="small"
              />
            )}
            sx={{ minWidth: 260 }}
          />
          {selectedItem && (
            <Chip
              size="small"
              color="primary"
              variant="outlined"
              label={`Brand: ${brandItemLabel(selectedItem)}`}
              onDelete={() => {
                setSelectedItem(null);
                setItemSearch("");
              }}
            />
          )}
          <TextField
            size="small"
            label="Portion"
            type="number"
            value={portion}
            slotProps={{ htmlInput: { min: 0, step: "any" } }}
            onChange={(e) => setPortion(e.target.value)}
          />
          <TextField
            size="small"
            label="Unit"
            value={unit}
            onChange={(e) => setUnit(e.target.value)}
          />
          <FormControlLabel
            control={
              <Checkbox
                checked={isOptional}
                onChange={(e) => setIsOptional(e.target.checked)}
              />
            }
            label="Optional"
          />
          <Button
            variant="contained"
            onClick={handleAddItem}
            disabled={
              (!selectedIngredient && !selectedItem) ||
              portion === "" ||
              Number(portion) <= 0
            }
          >
            Add Ingredient
          </Button>
        </Box>
        )}

        {addItemMutation.error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {(addItemMutation.error as Error).message}
          </Alert>
        )}
        {removeItemMutation.error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {(removeItemMutation.error as Error).message}
          </Alert>
        )}

        {recipeItemsQuery.isLoading && <CircularProgress />}
        {recipeItemsQuery.error && (
          <Alert severity="error">
            {(recipeItemsQuery.error as Error).message}
          </Alert>
        )}
        {!recipeItemsQuery.isLoading &&
          !recipeItemsQuery.error &&
          (recipeItemsQuery.data ?? []).length === 0 && (
            <Typography color="text.secondary">No ingredients</Typography>
          )}
        {(recipeItemsQuery.data ?? []).length > 0 && (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Portion</TableCell>
                  <TableCell>Item</TableCell>
                  <TableCell>Optional</TableCell>
                  <TableCell>Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {(recipeItemsQuery.data ?? []).map((recipeItem) => (
                  <Fragment key={recipeItem.recipeItemID ?? `add-${recipeItemLabel(recipeItem)}`}>
                  <TableRow>
                    <TableCell>
                      {fmtQty(recipeItem.quantity)}{" "}
                      {recipeItem.unitOfMeasure ?? ""}
                    </TableCell>
                    <TableCell>
                      {recipeItemLabel(recipeItem)}
                      {recipeItem.deltaKind && (
                        <DeltaBadge label={deltaBadgeLabel(recipeItem.deltaKind)} />
                      )}
                    </TableCell>
                    <TableCell>{recipeItem.isOptional ? "Yes" : "No"}</TableCell>
                    <TableCell>
                      {(recipeItem.recipeItemID ?? 0) > 0 && !!me?.household && (
                        <Button
                          size="small"
                          onClick={() =>
                            setTweakItemId(
                              tweakItemId === recipeItem.recipeItemID
                                ? null
                                : (recipeItem.recipeItemID ?? null)
                            )
                          }
                        >
                          Tweak
                        </Button>
                      )}
                      {isAdmin && (view === "canonical" || !delta) && (
                        <Button
                          size="small"
                          color="error"
                          onClick={() =>
                            removeItemMutation.mutate(recipeItem)
                          }
                        >
                          Remove
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                  {tweakItemId === recipeItem.recipeItemID &&
                    (recipeItem.recipeItemID ?? 0) > 0 && (
                      <TableRow>
                        <TableCell colSpan={4} sx={{ py: 0.5 }}>
                          <ItemTweakEditor
                            item={recipeItem}
                            existing={draftForLine(
                              draft.items,
                              recipeItem.recipeItemID ?? 0
                            )}
                            units={unitsQuery.data ?? []}
                            onApply={(row) => {
                              setDraft((d) => ({
                                ...d,
                                items: upsertItemDraft(d.items, row),
                              }));
                              setTweakItemId(null);
                            }}
                            onCancel={() => setTweakItemId(null)}
                          />
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Paper>

      {!!me?.household && (
        <RecipeDeltaPanel
          recipeId={recipeId}
          delta={delta}
          draft={draft}
          onDraftChange={setDraft}
          canonicalItems={canonicalQuery.data?.recipeItems ?? []}
          canonicalSteps={canonicalQuery.data?.recipeSteps ?? []}
        />
      )}

      <Paper sx={{ p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Steps
        </Typography>

        {isAdmin && (
        <>
        <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", mb: 2 }}>
          <FormControl size="small" sx={{ minWidth: 160 }}>
            <InputLabel id="step-position-label">Position</InputLabel>
            <Select
              labelId="step-position-label"
              label="Position"
              value={stepPosition}
              onChange={(e) => setStepPosition(e.target.value)}
            >
              <MenuItem value="beginning">At the beginning</MenuItem>
              {canonicalSteps
                .filter((s) => s.recipeStepID !== editingStepId)
                .map((s) => (
                  <MenuItem key={s.recipeStepID} value={`after-${s.recipeStepID}`}>
                    After step {s.stepNumber}
                  </MenuItem>
                ))}
              <MenuItem value="end">At the end</MenuItem>
            </Select>
          </FormControl>
          <TextField
            size="small"
            label="Instruction"
            sx={{ flexGrow: 1, minWidth: 260 }}
            value={instruction}
            onChange={(e) => setInstruction(e.target.value)}
          />
          <Button
            variant="contained"
            onClick={handleSaveStep}
            disabled={instruction.trim() === "" || saveStepsMutation.isPending}
          >
            {editingStepId === null ? "Add Step" : "Save Step"}
          </Button>
          {editingStepId !== null && (
            <Button onClick={resetStepForm}>Cancel</Button>
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", mb: 2, alignItems: "center" }}>
          <TextField
            size="small"
            label="Duration (min)"
            type="number"
            value={stepDuration}
            onChange={(e) => setStepDuration(e.target.value)}
            helperText="Feeds the event timeline"
          />
          <FormControl size="small" sx={{ minWidth: 140 }}>
            <InputLabel id="step-type-label">Step Type</InputLabel>
            <Select
              labelId="step-type-label"
              label="Step Type"
              value={stepType}
              onChange={(e) => setStepType(e.target.value)}
            >
              <MenuItem value="">
                <em>None</em>
              </MenuItem>
              {["prep", "cook", "rest", "wait", "serve", "other"].map((t) => (
                <MenuItem key={t} value={t}>
                  {t}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <FormControlLabel
            control={
              <Checkbox
                checked={stepPassive}
                onChange={(e) => setStepPassive(e.target.checked)}
              />
            }
            label="Passive (hands-free)"
          />
          <TextField
            size="small"
            label="Depends On Step"
            type="number"
            value={stepDependsOn}
            onChange={(e) => setStepDependsOn(e.target.value)}
            helperText="Defaults to previous step"
          />
          <TextField
            size="small"
            label="Appliance"
            value={stepAppliance}
            onChange={(e) => setStepAppliance(e.target.value)}
            helperText='e.g. "oven", "stovetop"'
          />
        </Box>
        </>
        )}

        {saveStepsMutation.error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {(saveStepsMutation.error as Error).message}
          </Alert>
        )}
        {deleteStepMutation.error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {(deleteStepMutation.error as Error).message}
          </Alert>
        )}

        {recipeStepsQuery.isLoading && <CircularProgress />}
        {recipeStepsQuery.error && (
          <Alert severity="error">
            {(recipeStepsQuery.error as Error).message}
          </Alert>
        )}
        {!recipeStepsQuery.isLoading &&
          !recipeStepsQuery.error &&
          sortedSteps.length === 0 && (
            <Typography color="text.secondary">No steps</Typography>
          )}
        {sortedSteps.map((step) => (
          <Box key={step.recipeStepID}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 2,
                py: 1,
                flexWrap: "wrap",
              }}
            >
              <Typography sx={{ minWidth: 32 }}>{step.stepNumber}.</Typography>
              <Typography sx={{ flex: "1 1 240px", minWidth: 0 }}>
                {step.instruction}
                {step.deltaKind && (
                  <DeltaBadge label={deltaBadgeLabel(step.deltaKind)} />
                )}
              </Typography>
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ minWidth: { xs: 0, sm: 180 } }}
              >
                {[
                  step.durationMinutes != null ? `${step.durationMinutes}m` : null,
                  step.stepType,
                  step.isPassive ? "passive" : null,
                  step.appliance,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </Typography>
              {step.recipeStepID > 0 && !!me?.household && (
                <Button
                  size="small"
                  onClick={() =>
                    setTweakStepId(
                      tweakStepId === step.recipeStepID ? null : step.recipeStepID
                    )
                  }
                >
                  Tweak
                </Button>
              )}
              {isAdmin && (view === "canonical" || !delta) && (
                <>
                  <Button size="small" onClick={() => handleEditStep(step)}>
                    Edit
                  </Button>
                  <Button
                    size="small"
                    color="error"
                    onClick={() => handleDeleteStep(step)}
                  >
                    Delete
                  </Button>
                </>
              )}
            </Box>
            {tweakStepId === step.recipeStepID && step.recipeStepID > 0 && (
              <StepTweakEditor
                step={step}
                existing={draftForStep(draft.steps, step.recipeStepID)}
                onApply={(row) => {
                  setDraft((d) => ({
                    ...d,
                    steps: upsertStepDraft(d.steps, row),
                  }));
                  setTweakStepId(null);
                }}
                onCancel={() => setTweakStepId(null)}
              />
            )}
            <Divider />
          </Box>
        ))}
      </Paper>
    </Box>
  );
}
