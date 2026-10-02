"use client";

import { useEffect, useState } from "react";
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
import Typography from "@mui/material/Typography";
import * as aiSuggest from "@/lib/ai/suggest";
import { useLocalEngineReady } from "@/lib/ai/engineStore";
import { api } from "@/lib/api";
import { Brand, PairingSuggestion, RecipeStep } from "@/lib/types";
import { fmtQty } from "@/lib/format";
import { isOfDrinkingAge } from "@/lib/age";
import { useMe } from "@/app/auth/useMe";

export default function RecipeDetailPage() {
  const params = useParams<{ id: string }>();
  const recipeId = Number(params.id);
  const queryClient = useQueryClient();

  const [itemId, setItemId] = useState<number | "">("");
  const [portion, setPortion] = useState("");
  const [unit, setUnit] = useState("");
  const [isOptional, setIsOptional] = useState(false);

  const [stepNumber, setStepNumber] = useState("");
  const [instruction, setInstruction] = useState("");
  const [stepDuration, setStepDuration] = useState("");
  const [stepType, setStepType] = useState("");
  const [stepPassive, setStepPassive] = useState(false);
  const [stepDependsOn, setStepDependsOn] = useState("");
  const [stepAppliance, setStepAppliance] = useState("");
  const [editingStepId, setEditingStepId] = useState<number | null>(null);
  const [brandId, setBrandId] = useState<number | null>(null);
  const [brandInput, setBrandInput] = useState("");
  const [debouncedBrandInput, setDebouncedBrandInput] = useState("");
  const [itemSearch, setItemSearch] = useState("");
  const [debouncedItemSearch, setDebouncedItemSearch] = useState("");

  const { me } = useMe();

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
  const canPair = (aiQuery.data === true || localAIReady) && isOfDrinkingAge(me?.birthdate);

  const recipeQuery = useQuery({
    queryKey: ["recipe", recipeId],
    queryFn: () => api.getRecipe(recipeId),
    enabled: !isNaN(recipeId),
  });

  const groupsQuery = useQuery({
    queryKey: ["recipe-category-groups"],
    queryFn: api.getRecipeCategoryGroups,
    staleTime: 60_000,
  });

  useEffect(() => {
    if (!isNaN(recipeId)) void api.recordView("recipe", recipeId);
  }, [recipeId]);

  const brandsQuery = useQuery({
    queryKey: ["item-brands", brandInput],
    queryFn: () =>
      brandInput === ""
        ? api.getFrequentBrands(10)
        : api.getBrands(brandInput),
  });

  const brandOptions = brandsQuery.data ?? [];
  const selectedBrand =
    brandId === null
      ? null
      : brandOptions.find((b) => b.brandID === brandId) ?? null;

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedItemSearch(itemSearch), 300);
    return () => clearTimeout(timer);
  }, [itemSearch]);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedBrandInput(brandInput), 300);
    return () => clearTimeout(timer);
  }, [brandInput]);

  useEffect(() => {
    if (debouncedItemSearch.trim()) {
      void api.recordSearch("item", debouncedItemSearch);
    }
  }, [debouncedItemSearch]);

  useEffect(() => {
    if (debouncedBrandInput.trim()) {
      void api.recordSearch("brand", debouncedBrandInput);
    }
  }, [debouncedBrandInput]);

  const searchQuery = useQuery({
    queryKey: ["items-search", debouncedItemSearch, brandId],
    queryFn: () =>
      api.searchItems(
        debouncedItemSearch,
        brandId ?? undefined,
        brandId !== null && debouncedItemSearch.length === 0 ? 100 : 50
      ),
    enabled: brandId !== null || debouncedItemSearch.length >= 2,
  });

  const recipeItemsQuery = useQuery({
    queryKey: ["recipe-items", recipeId],
    queryFn: () => api.getRecipeItems(recipeId),
    enabled: !isNaN(recipeId),
  });

  const recipeStepsQuery = useQuery({
    queryKey: ["recipe-steps", recipeId],
    queryFn: () => api.getRecipeSteps(recipeId),
    enabled: !isNaN(recipeId),
  });

  const invalidateItems = () =>
    queryClient.invalidateQueries({ queryKey: ["recipe-items", recipeId] });
  const invalidateSteps = () =>
    queryClient.invalidateQueries({ queryKey: ["recipe-steps", recipeId] });

  const addItemMutation = useMutation({
    mutationFn: (payload: {
      itemId: number;
      portion: number;
      unit: string | null;
      isOptional: boolean;
    }) => api.addRecipeItem(recipeId, payload),
    onSuccess: () => {
      setItemId("");
      setItemSearch("");
      setDebouncedItemSearch("");
      setPortion("");
      setUnit("");
      setIsOptional(false);
      return invalidateItems();
    },
  });

  const removeItemMutation = useMutation({
    mutationFn: (id: number) => api.removeRecipeItem(recipeId, id),
    onSuccess: invalidateItems,
  });

  const addStepMutation = useMutation({
    mutationFn: (payload: Partial<RecipeStep> & { stepNumber: number; instruction: string }) =>
      api.addRecipeStep(recipeId, payload),
    onSuccess: () => {
      resetStepForm();
      return invalidateSteps();
    },
  });

  const updateStepMutation = useMutation({
    mutationFn: ({
      stepId,
      ...payload
    }: {
      stepId: number;
    } & Partial<RecipeStep> & { stepNumber: number; instruction: string }) =>
      api.updateRecipeStep(recipeId, stepId, payload),
    onSuccess: () => {
      resetStepForm();
      return invalidateSteps();
    },
  });

  const deleteStepMutation = useMutation({
    mutationFn: (stepId: number) => api.deleteRecipeStep(recipeId, stepId),
    onSuccess: invalidateSteps,
  });

  const rateMutation = useMutation({
    mutationFn: (rating: number) => api.rateRecipe(recipeId, rating),
    onSuccess: (updated) => {
      queryClient.setQueryData(["recipe", recipeId], updated);
    },
  });

  const setCategoriesMutation = useMutation({
    mutationFn: (categoryIds: number[]) =>
      api.setRecipeCategories(recipeId, categoryIds),
    onSuccess: (updated) => {
      queryClient.setQueryData(["recipe", recipeId], updated);
      queryClient.invalidateQueries({ queryKey: ["recipes"] });
    },
  });

  const resetStepForm = () => {
    setEditingStepId(null);
    setStepNumber("");
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
    if (itemId === "" || portion === "") return;
    addItemMutation.mutate({
      itemId: Number(itemId),
      portion: Number(portion),
      unit: unit === "" ? null : unit,
      isOptional,
    });
  };

  const handleSaveStep = () => {
    if (stepNumber === "" || instruction.trim() === "") return;
    if (editingStepId === null) {
      addStepMutation.mutate({
        stepNumber: Number(stepNumber),
        instruction,
        ...stepTimingFields(),
      });
    } else {
      updateStepMutation.mutate({
        stepId: editingStepId,
        stepNumber: Number(stepNumber),
        instruction,
        ...stepTimingFields(),
      });
    }
  };

  const handleEditStep = (step: RecipeStep) => {
    setEditingStepId(step.recipeStepID);
    setStepNumber(String(step.stepNumber));
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

  if (isNaN(recipeId)) {
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
            <Typography variant="h4" gutterBottom>
              {recipeQuery.data.recipeName}
            </Typography>
            <Typography color="text.secondary" gutterBottom>
              {recipeQuery.data.description ?? "No description"}
            </Typography>
            <Typography variant="body2">
              Servings: {recipeQuery.data.servings ?? "-"}
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 1 }}>
              <Rating
                aria-label="Your rating"
                value={recipeQuery.data.myRating}
                onChange={(_, value) => {
                  if (value != null) rateMutation.mutate(value);
                }}
              />
              <Typography variant="body2" color="text.secondary">
                {recipeQuery.data.averageRating != null
                  ? `${recipeQuery.data.averageRating.toFixed(1)} avg · ${recipeQuery.data.ratingCount} rating${recipeQuery.data.ratingCount === 1 ? "" : "s"}`
                  : "No ratings yet"}
              </Typography>
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
            {rateMutation.error && (
              <Alert severity="error" sx={{ mt: 1 }}>
                {(rateMutation.error as Error).message}
              </Alert>
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

      <Paper sx={{ p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Ingredients
        </Typography>

        <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", mb: 2 }}>
          <Autocomplete
            size="small"
            options={brandOptions}
            getOptionLabel={(b) => (typeof b === "string" ? b : b?.brandName ?? "")}
            isOptionEqualToValue={(a, b) =>
              (typeof a === "object" && typeof b === "object" && a?.brandID === b?.brandID)
            }
            inputValue={brandInput}
            onInputChange={(_, value) => setBrandInput(value)}
            value={selectedBrand}
            onChange={(_, value) => {
              const b = value as Brand | null;
              setBrandId(b?.brandID ?? null);
              setBrandInput(b?.brandName ?? "");
              setItemId("");
              setItemSearch("");
              setDebouncedItemSearch("");
              if (b) void api.recordSelection("brand", b.brandID);
            }}
            filterOptions={(options) => options}
            loading={brandsQuery.isLoading}
            noOptionsText="No brands found"
            renderInput={(params) => (
              <TextField {...params} label="Brand" size="small" />
            )}
            sx={{ minWidth: 180 }}
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
            value={
              searchQuery.data?.find((item) => item.itemID === itemId) ?? null
            }
            onChange={(_, value) => {
              setItemId(value ? value.itemID : "");
              if (value) void api.recordSelection("item", value.itemID);
            }}
            filterOptions={(options) => options}
            loading={searchQuery.isLoading}
            noOptionsText={
              brandId === null && debouncedItemSearch.length < 2
                ? "Type at least 2 characters"
                : brandId !== null && debouncedItemSearch.length === 0
                ? "No items for this brand"
                : "No items found"
            }
            renderOption={(props, item) => {
              const { key, ...liProps } = props;
              return (
                <li key={item.itemID} {...liProps}>
                  {item.name}
                  {item.brand ? ` — ${item.brand}` : ""}
                </li>
              );
            }}
            renderInput={(params) => (
              <TextField {...params} label="Item" size="small" />
            )}
            sx={{ minWidth: 260 }}
          />
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
            disabled={itemId === "" || portion === "" || Number(portion) <= 0}
          >
            Add Item
          </Button>
        </Box>

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
                  <TableRow key={recipeItem.itemID}>
                    <TableCell>
                      {fmtQty(recipeItem.quantity)}{" "}
                      {recipeItem.unitOfMeasure ?? ""}
                    </TableCell>
                    <TableCell>
                      {recipeItem.itemBrand
                        ? `${recipeItem.itemBrand} — ${recipeItem.itemName ?? recipeItem.itemID}`
                        : (recipeItem.itemName ?? recipeItem.itemID)}
                    </TableCell>
                    <TableCell>{recipeItem.isOptional ? "Yes" : "No"}</TableCell>
                    <TableCell>
                      <Button
                        size="small"
                        color="error"
                        onClick={() =>
                          removeItemMutation.mutate(recipeItem.itemID)
                        }
                      >
                        Remove
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Paper>

      <Paper sx={{ p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Steps
        </Typography>

        <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", mb: 2 }}>
          <TextField
            size="small"
            label="Step Number"
            type="number"
            value={stepNumber}
            onChange={(e) => setStepNumber(e.target.value)}
          />
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
            disabled={stepNumber === "" || instruction.trim() === ""}
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

        {addStepMutation.error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {(addStepMutation.error as Error).message}
          </Alert>
        )}
        {updateStepMutation.error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {(updateStepMutation.error as Error).message}
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
              }}
            >
              <Typography sx={{ minWidth: 32 }}>{step.stepNumber}.</Typography>
              <Typography sx={{ flexGrow: 1 }}>
                {step.instruction}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ minWidth: 180 }}>
                {[
                  step.durationMinutes != null ? `${step.durationMinutes}m` : null,
                  step.stepType,
                  step.isPassive ? "passive" : null,
                  step.appliance,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </Typography>
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
            </Box>
            <Divider />
          </Box>
        ))}
      </Paper>
    </Box>
  );
}
