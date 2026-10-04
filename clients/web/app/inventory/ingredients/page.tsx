"use client";

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Autocomplete from "@mui/material/Autocomplete";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { api } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";
import IngredientAutocomplete from "@/app/components/IngredientAutocomplete";
import AllergenFlagsEditor from "@/app/components/AllergenFlagsEditor";
import { AllergenFlag, Category, Ingredient } from "@/lib/types";

const ingredientFields: FieldDef<Ingredient>[] = [
  { key: "name", label: "Name" },
  { key: "defaultUnit", label: "Default Unit" },
  { key: "isActive", label: "Active", type: "boolean" },
];

// Admin catalog for generic ingredients — the identity recipes and
// grocery needs key on. Merge repoints every reference from the source
// row onto a survivor, then deletes the source.
export default function IngredientsPage() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogData, setDialogData] = useState<Record<string, unknown>>({});
  const [isCreate, setIsCreate] = useState(false);
  const [dialogError, setDialogError] = useState<Error | null>(null);
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  // undefined = untouched; null = clear the category.
  const [dialogCategory, setDialogCategory] = useState<Category | null | undefined>(undefined);
  const [mergeSource, setMergeSource] = useState<Ingredient | null>(null);
  const [mergeTarget, setMergeTarget] = useState<Ingredient | null>(null);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    // eslint-disable-next-line @eslint-react/set-state-in-effect
    setPageNumber(1);
  }, [debouncedSearch]);

  const listQuery = useQuery({
    queryKey: ["ingredients", pageNumber, pageSize, debouncedSearch],
    queryFn: () => api.getIngredients(pageNumber, pageSize, debouncedSearch || undefined),
    placeholderData: (prev) => prev,
  });

  const categoriesQuery = useQuery({
    queryKey: ["categories"],
    queryFn: api.getCategories,
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["ingredients"] });

  const saveMutation = useMutation({
    mutationFn: (values: Record<string, unknown>) => {
      const categoryId =
        dialogCategory === undefined
          ? undefined
          : (dialogCategory?.categoryID ?? null);
      if (isCreate) {
        return api.createIngredient({
          name: String(values.name ?? ""),
          categoryId: categoryId ?? null,
          defaultUnit: (values.defaultUnit as string) || null,
        });
      }
      return api.updateIngredient(values.ingredientID as number, {
        name: values.name as string,
        ...(categoryId !== undefined ? { categoryId } : {}),
        defaultUnit: (values.defaultUnit as string) || null,
        isActive: Boolean(values.isActive),
      });
    },
    onSuccess: () => {
      setDialogError(null);
      setDialogOpen(false);
      void refresh();
    },
    onError: (err: unknown) => setDialogError(err as Error),
  });

  const deleteMutation = useMutation({
    mutationFn: (row: Ingredient) => api.deleteIngredient(row.ingredientID),
    onSuccess: refresh,
  });

  const mergeMutation = useMutation({
    mutationFn: ({ fromId, intoId }: { fromId: number; intoId: number }) =>
      api.mergeIngredient(fromId, intoId),
    onSuccess: () => {
      setMergeSource(null);
      setMergeTarget(null);
      void refresh();
    },
  });

  const handleCreate = () => {
    setIsCreate(true);
    setDialogData({});
    setDialogCategory(undefined);
    setDialogError(null);
    setDialogOpen(true);
  };

  const handleEdit = (row: Ingredient) => {
    setIsCreate(false);
    setDialogData({ ...row });
    setDialogCategory(undefined);
    setDialogError(null);
    setDialogOpen(true);
  };

  const handleDelete = (row: Ingredient) => {
    if (window.confirm(`Delete "${row.name}"?`)) {
      deleteMutation.mutate(row);
    }
  };

  const columns: FieldDef<Ingredient>[] = [
    { key: "name", label: "Name", sortable: true },
    {
      key: "category",
      label: "Category",
      render: (row) => row.category?.categoryName ?? "—",
    },
    { key: "defaultUnit", label: "Default Unit" },
    { key: "isActive", label: "Active", type: "boolean", sortable: true },
  ];

  const extraActions = (row: Ingredient) => (
    <Button size="small" onClick={() => {
      setMergeSource(row);
      setMergeTarget(null);
    }}>
      Merge into…
    </Button>
  );

  return (
    <Box>
      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", mb: 2 }}>
        <TextField
          size="small"
          label="Search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          sx={{ minWidth: 240 }}
        />
      </Box>
      <DataTable
        title="Ingredients"
        rows={listQuery.data?.items ?? []}
        isLoading={listQuery.isLoading}
        error={listQuery.error as Error | null}
        onCreate={handleCreate}
        onEdit={handleEdit}
        onDelete={handleDelete}
        extraActions={extraActions}
        fields={columns}
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
        title={isCreate ? "Create Ingredient" : "Edit Ingredient"}
        fields={ingredientFields}
        values={dialogData}
        error={dialogError}
        extraFields={
          <>
            <Autocomplete<Category>
              size="small"
              options={categoriesQuery.data ?? []}
              getOptionLabel={(c) => c.categoryName}
              isOptionEqualToValue={(o, v) => o.categoryID === v.categoryID}
              value={
                dialogCategory !== undefined
                  ? dialogCategory
                  : ((dialogData.category as Category | null) ?? null)
              }
              onChange={(_, v) => setDialogCategory(v)}
              renderInput={(params) => (
                <TextField {...params} label="Category" margin="dense" fullWidth />
              )}
            />
            <AllergenFlagsEditor
              key={isCreate ? "new" : (dialogData.ingredientID as number)}
              flags={dialogData.allergens as AllergenFlag[] | undefined}
              disabledReason={
                isCreate ? "Save the ingredient first to set allergen flags." : undefined
              }
              onSet={async (allergenID, kind) => {
                await api.setIngredientAllergen(
                  dialogData.ingredientID as number,
                  allergenID,
                  kind
                );
                void refresh();
              }}
            />
          </>
        }
        onClose={() => setDialogOpen(false)}
        onSave={(values) => saveMutation.mutate(values)}
      />
      <Dialog
        open={mergeSource !== null}
        onClose={() => setMergeSource(null)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Merge &quot;{mergeSource?.name}&quot; into…</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Every recipe line, meal-slot item, grocery row, aisle route,
            item link, and household preference pointing at
            &quot;{mergeSource?.name}&quot; moves to the ingredient you pick —
            then &quot;{mergeSource?.name}&quot; is deleted. This can&apos;t be
            undone.
          </Typography>
          <IngredientAutocomplete
            value={mergeTarget}
            onChange={setMergeTarget}
            label="Surviving ingredient"
          />
          {mergeMutation.error && (
            <Typography color="error" variant="body2" sx={{ mt: 1 }}>
              {(mergeMutation.error as Error).message}
            </Typography>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setMergeSource(null)}>Cancel</Button>
          <Button
            variant="contained"
            color="warning"
            disabled={!mergeTarget || mergeMutation.isPending}
            onClick={() => {
              if (mergeSource && mergeTarget) {
                mergeMutation.mutate({
                  fromId: mergeSource.ingredientID,
                  intoId: mergeTarget.ingredientID,
                });
              }
            }}
          >
            Merge
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
