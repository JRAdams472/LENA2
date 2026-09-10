"use client";

import { useEffect, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import FormControlLabel from "@mui/material/FormControlLabel";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import Link from "next/link";
import { api, asEntity, ApiError } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";
import { Recipe } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";

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
  const { isAdmin } = useMe();
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogData, setDialogData] = useState<Record<string, unknown>>({});
  const [isCreate, setIsCreate] = useState(false);
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [isFavorite, setIsFavorite] = useState(false);

  const [uploadOpen, setUploadOpen] = useState(false);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploadSuccess, setUploadSuccess] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

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

  const fileToBase64 = (file: File): Promise<string> =>
    new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result as string);
      reader.onerror = reject;
      reader.readAsDataURL(file);
    });

  const handleUploadClick = () => {
    setUploadError(null);
    setUploadSuccess(null);
    setUploadOpen(true);
  };

  const handleUploadFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setUploadFile(e.target.files?.[0] ?? null);
    setUploadError(null);
  };

  const handleUpload = async () => {
    if (!uploadFile) return;
    setUploading(true);
    setUploadError(null);
    try {
      const fileBase64 = await fileToBase64(uploadFile);
      await api.submitRecipeScan(fileBase64);
      setUploadSuccess(`Saved ${uploadFile.name} to the import inbox. Run the ocrimport CLI pipeline to extract it.`);
      setUploadFile(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
      setUploadOpen(false);
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : "Failed to upload recipe scan";
      setUploadError(msg);
    } finally {
      setUploading(false);
    }
  };

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
      {uploadSuccess && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setUploadSuccess(null)}>
          {uploadSuccess}
        </Alert>
      )}
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
        {isAdmin && (
          <Button variant="outlined" onClick={handleUploadClick}>
            Upload Recipe Scan
          </Button>
        )}
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
      <Dialog open={uploadOpen} onClose={() => setUploadOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>Upload Recipe Scan</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{ mb: 2 }}>
            Choose a PNG, JPG, or PDF recipe scan. The file is written to the
            import inbox for the admin CLI pipeline (ocrimport).
          </DialogContentText>
          {uploadError && (
            <Alert severity="error" sx={{ mb: 2 }} onClose={() => setUploadError(null)}>
              {uploadError}
            </Alert>
          )}
          <Button
            variant="outlined"
            component="label"
            disabled={uploading}
          >
            Choose File
            <input
              ref={fileInputRef}
              type="file"
              accept=".png,.jpg,.jpeg,.pdf"
              hidden
              onChange={handleUploadFileChange}
            />
          </Button>
          {uploadFile && (
            <Box sx={{ mt: 2 }}>
              Selected: <strong>{uploadFile.name}</strong>
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setUploadOpen(false)} disabled={uploading}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleUpload}
            disabled={!uploadFile || uploading}
            startIcon={uploading ? <CircularProgress size={16} /> : null}
          >
            {uploading ? "Uploading..." : "Upload"}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
