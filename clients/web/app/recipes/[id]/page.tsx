"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import CircularProgress from "@mui/material/CircularProgress";
import FormControlLabel from "@mui/material/FormControlLabel";
import Divider from "@mui/material/Divider";
import Paper from "@mui/material/Paper";
import Autocomplete from "@mui/material/Autocomplete";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { api } from "@/lib/api";
import { RecipeStep } from "@/lib/types";

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
  const [editingStepId, setEditingStepId] = useState<number | null>(null);
  const [brand, setBrand] = useState<string | "">("");
  const [brandInput, setBrandInput] = useState("");
  const [itemSearch, setItemSearch] = useState("");
  const [debouncedItemSearch, setDebouncedItemSearch] = useState("");

  const recipeQuery = useQuery({
    queryKey: ["recipe", recipeId],
    queryFn: () => api.getRecipe(recipeId),
    enabled: !isNaN(recipeId),
  });

  const brandsQuery = useQuery({
    queryKey: ["item-brands", brandInput],
    queryFn: () => api.getBrands(brandInput),
  });

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedItemSearch(itemSearch), 300);
    return () => clearTimeout(timer);
  }, [itemSearch]);

  const searchQuery = useQuery({
    queryKey: ["items-search", debouncedItemSearch, brand],
    queryFn: () =>
      api.searchItems(
        debouncedItemSearch,
        brand,
        brand !== "" && debouncedItemSearch.length === 0 ? 1000 : 50
      ),
    enabled: brand !== "" || debouncedItemSearch.length >= 2,
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
    mutationFn: (payload: { stepNumber: number; instruction: string }) =>
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
      stepNumber: number;
      instruction: string;
    }) => api.updateRecipeStep(recipeId, stepId, payload),
    onSuccess: () => {
      resetStepForm();
      return invalidateSteps();
    },
  });

  const deleteStepMutation = useMutation({
    mutationFn: (stepId: number) => api.deleteRecipeStep(recipeId, stepId),
    onSuccess: invalidateSteps,
  });

  const resetStepForm = () => {
    setEditingStepId(null);
    setStepNumber("");
    setInstruction("");
  };

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
      });
    } else {
      updateStepMutation.mutate({
        stepId: editingStepId,
        stepNumber: Number(stepNumber),
        instruction,
      });
    }
  };

  const handleEditStep = (step: RecipeStep) => {
    setEditingStepId(step.recipeStepID);
    setStepNumber(String(step.stepNumber));
    setInstruction(step.instruction);
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
          </>
        )}
      </Paper>

      <Paper sx={{ p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Ingredients
        </Typography>

        <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", mb: 2 }}>
          <Autocomplete
            size="small"
            options={brandsQuery.data ?? []}
            getOptionLabel={(b) => b ?? ""}
            isOptionEqualToValue={(a, b) => a === b}
            inputValue={brandInput}
            onInputChange={(_, value) => setBrandInput(value)}
            value={brand === "" ? null : brand}
            onChange={(_, value) => {
              setBrand(value ?? "");
              setBrandInput(value ?? "");
              setItemId("");
              setItemSearch("");
              setDebouncedItemSearch("");
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
            onChange={(_, value) => setItemId(value ? value.itemID : "")}
            filterOptions={(options) => options}
            loading={searchQuery.isLoading}
            noOptionsText={
              brand === "" && debouncedItemSearch.length < 2
                ? "Type at least 2 characters"
                : brand !== "" && debouncedItemSearch.length === 0
                ? "No items for this brand"
                : "No items found"
            }
            renderOption={(props, item) => (
              <li {...props} key={item.itemID}>
                {item.name}
                {item.brand ? ` — ${item.brand}` : ""}
              </li>
            )}
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
            disabled={itemId === "" || portion === ""}
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
                      {recipeItem.quantity}{" "}
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
              <Typography sx={{ flexGrow: 1 }}>{step.instruction}</Typography>
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
