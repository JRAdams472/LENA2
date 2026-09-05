"use client";

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import TextField from "@mui/material/TextField";
import Switch from "@mui/material/Switch";
import FormControlLabel from "@mui/material/FormControlLabel";
import Link from "next/link";
import { api, asEntity } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";
import { Recipe } from "@/lib/types";

function toRow(recipe: Recipe) {
  return {
    recipeID: recipe.recipeID,
    recipeName: recipe.recipeName,
    description: recipe.description,
    prepTimeMinutes: recipe.prepTimeMinutes,
    cookTimeMinutes: recipe.cookTimeMinutes,
    isActive: recipe.isActive,
  };
}

type RecipeRow = ReturnType<typeof toRow>;

const recipeTableFields: FieldDef<RecipeRow>[] = [
  { key: "recipeName", label: "Name" },
  { key: "description", label: "Description" },
  { key: "prepTimeMinutes", label: "Prep Time", type: "number" },
  { key: "cookTimeMinutes", label: "Cook Time", type: "number" },
  { key: "isActive", label: "Active", type: "boolean" },
];

const recipeFields: FieldDef<Recipe>[] = [
  ...(recipeTableFields as FieldDef<Recipe>[]),
  { key: "servings", label: "Servings", type: "number" },
  { key: "isFavorite", label: "Favorite", type: "boolean" },
];

export default function RecipesPage() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogData, setDialogData] = useState<Record<string, unknown>>({});
  const [isCreate, setIsCreate] = useState(false);
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [isFavorite, setIsFavorite] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPageNumber(1);
  }, [debouncedSearch, isFavorite]);

  const listQuery = useQuery({
    queryKey: ["recipes", pageNumber, pageSize, debouncedSearch, isFavorite],
    queryFn: () => api.getRecipesPaged(pageNumber, pageSize, debouncedSearch, isFavorite),
    placeholderData: (prev) => prev,
  });

  const createMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.createRecipe(asEntity(row)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["recipes"] }),
  });

  const updateMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.updateRecipe(row.recipeID as number, asEntity(row)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["recipes"] }),
  });

  const deleteMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.deleteRecipe(row.recipeID as number),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["recipes"] }),
  });

  const handleCreate = () => {
    setIsCreate(true);
    setDialogData({});
    setDialogOpen(true);
  };

  const handleEdit = (row: RecipeRow) => {
    const full = listQuery.data?.items.find((r) => r.recipeID === row.recipeID);
    if (!full) return;
    setIsCreate(false);
    setDialogData({ ...full });
    setDialogOpen(true);
  };

  const handleDelete = (row: Record<string, unknown>) => {
    if (window.confirm("Delete this recipe?")) {
      deleteMutation.mutate(row);
    }
  };

  const handleSave = (values: Record<string, unknown>) => {
    if (isCreate) {
      createMutation.mutate(values);
    } else {
      updateMutation.mutate(values);
    }
    setDialogOpen(false);
  };

  const extraActions = (row: Record<string, unknown>) => (
    <Button
      size="small"
      component={Link}
      href={`/recipes/${row.recipeID as number}`}
    >
      Manage
    </Button>
  );

  return (
    <Box>
      <Box sx={{ display: "flex", gap: 2, alignItems: "center", mb: 2 }}>
        <TextField
          size="small"
          label="Search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          sx={{ minWidth: 260 }}
        />
        <FormControlLabel
          control={
            <Switch
              checked={isFavorite}
              onChange={(e) => setIsFavorite(e.target.checked)}
            />
          }
          label="Favorites"
        />
      </Box>
      <DataTable
        title="Recipes"
        rows={(listQuery.data?.items ?? []).map(toRow)}
        isLoading={listQuery.isLoading}
        error={listQuery.error as Error | null}
        onCreate={handleCreate}
        onEdit={handleEdit}
        onDelete={handleDelete}
        extraActions={extraActions}
        fields={recipeTableFields}
        pagination={
          listQuery.data
            ? {
                pageNumber,
                pageSize,
                totalCount: listQuery.data.totalCount,
                totalPages: listQuery.data.totalPages,
                onPageChange: setPageNumber,
                onPageSizeChange: (size) => { setPageSize(size); setPageNumber(1); },
              }
            : undefined
        }
      />
      <CrudDialog
        open={dialogOpen}
        title={isCreate ? "Create Recipe" : "Edit Recipe"}
        fields={recipeFields}
        values={dialogData}
        onClose={() => setDialogOpen(false)}
        onSave={handleSave}
      />
    </Box>
  );
}
