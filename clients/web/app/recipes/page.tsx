"use client";

import { useEffect, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
import InputLabel from "@mui/material/InputLabel";
import ListItemText from "@mui/material/ListItemText";
import MenuItem from "@mui/material/MenuItem";
import OutlinedInput from "@mui/material/OutlinedInput";
import Select from "@mui/material/Select";
import Switch from "@mui/material/Switch";
import AutoAwesomeIcon from "@mui/icons-material/AutoAwesome";
import TextField from "@mui/material/TextField";
import Link from "next/link";
import * as aiSuggest from "@/lib/ai/suggest";
import { useLocalEngineReady } from "@/lib/ai/engineStore";
import { api, asEntity, ApiError } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";
import { AllergyWarningChip } from "@/app/components/AllergyWarning";
import { CocktailSuggestion, Recipe } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";
import { isOfDrinkingAge } from "@/lib/age";
import Chip from "@mui/material/Chip";

function toRow(recipe: Recipe) {
  return {
    recipeID: recipe.recipeID,
    recipeName: recipe.recipeName,
    description: recipe.description,
    prepTimeMinutes: recipe.prepTimeMinutes,
    cookTimeMinutes: recipe.cookTimeMinutes,
    isActive: recipe.isActive,
    allergyWarnings: recipe.allergyWarnings,
  };
}

type RecipeRow = ReturnType<typeof toRow>;

const recipeTableFields: FieldDef<RecipeRow>[] = [
  { key: "recipeName", label: "Name" },
  { key: "description", label: "Description" },
  {
    key: "allergyWarnings",
    label: "Warnings",
    render: (row) => <AllergyWarningChip warnings={row.allergyWarnings} />,
  },
  {
    key: "prepTimeMinutes",
    label: "Prep Time",
    // type drives the dialog input; render drives the table cell.
    type: "number",
    render: (r) => (r.prepTimeMinutes == null ? "—" : `${r.prepTimeMinutes} min`),
  },
  {
    key: "cookTimeMinutes",
    label: "Cook Time",
    type: "number",
    render: (r) => (r.cookTimeMinutes == null ? "—" : `${r.cookTimeMinutes} min`),
  },
  { key: "isActive", label: "Active", type: "boolean" },
];

const recipeFields: FieldDef<Recipe>[] = [
  ...(recipeTableFields as FieldDef<Recipe>[]),
  { key: "servings", label: "Servings", type: "number" },
  { key: "isFavorite", label: "Favorite", type: "boolean" },
];

export default function RecipesPage() {
  const { isAdmin, me } = useMe();
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogData, setDialogData] = useState<Record<string, unknown>>({});
  const [isCreate, setIsCreate] = useState(false);
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [isFavorite, setIsFavorite] = useState(false);
  const [semantic, setSemantic] = useState(false);
  const [categoryIds, setCategoryIds] = useState<number[]>([]);

  const [uploadOpen, setUploadOpen] = useState(false);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploadSuccess, setUploadSuccess] = useState<string | null>(null);
  const [uploadImportId, setUploadImportId] = useState<number | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(search);
      const term = search.trim();
      if (term) void api.recordSearch("recipe", term);
    }, 300);
    return () => clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    // eslint-disable-next-line @eslint-react/set-state-in-effect
    setPageNumber(1);
  }, [debouncedSearch, isFavorite, semantic, categoryIds]);

  const groupsQuery = useQuery({
    queryKey: ["recipe-category-groups"],
    queryFn: api.getRecipeCategoryGroups,
    staleTime: 60_000,
  });

  const localAIReady = useLocalEngineReady();
  const aiQuery = useQuery({
    queryKey: ["aiAvailable"],
    queryFn: () => api.getAIAvailable(),
    staleTime: 5 * 60 * 1000,
  });

  const semanticQuery = useQuery({
    queryKey: ["semanticSearchAvailable"],
    queryFn: () => api.getSemanticSearchAvailable(),
    staleTime: 5 * 60 * 1000,
  });
  const semanticAvailable = semanticQuery.data === true;
  const semanticIdle = semantic && debouncedSearch.trim() === "";

  const [cocktailsOpen, setCocktailsOpen] = useState(false);
  const [inStockOnly, setInStockOnly] = useState(false);
  const cocktailMutation = useMutation({
    mutationFn: (stockOnly: boolean) => aiSuggest.suggestCocktails(stockOnly),
  });
  const canSuggestCocktails =
    (aiQuery.data === true || localAIReady) && isOfDrinkingAge(me?.birthdate);

  const listQuery = useQuery({
    queryKey: ["recipes", pageNumber, pageSize, debouncedSearch, isFavorite, semantic, categoryIds],
    queryFn: () =>
      api.getRecipesPaged(pageNumber, pageSize, debouncedSearch, isFavorite, categoryIds, semantic ? "semantic" : "keyword"),
    placeholderData: (prev) => prev,
    // With semantic on and nothing typed yet there's no query to embed —
    // the hint below replaces the list instead.
    enabled: !semanticIdle,
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
    setUploadSuccess(null);
    setUploadImportId(null);
    try {
      const fileBase64 = await fileToBase64(uploadFile);
      const imported = await api.submitRecipeScan(fileBase64);
      setUploadImportId(imported.recipeImportID);
      setUploadSuccess(`Saved ${uploadFile.name} and enqueued import ${imported.recipeImportID} for OCR processing.`);
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
          {uploadSuccess}{" "}
          {uploadImportId !== null && (
            <Link href={`/recipes/pending/${uploadImportId}`}>Review import</Link>
          )}
        </Alert>
      )}
      <Box
        sx={{
          display: "flex",
          gap: 2,
          alignItems: "center",
          flexWrap: "wrap",
          mb: 2,
        }}
      >
        <TextField
          size="small"
          label="Search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          sx={{ minWidth: 200, flexGrow: { xs: 1, sm: 0 } }}
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
        {semanticAvailable && (
          <FormControlLabel
            control={
              <Switch
                checked={semantic}
                onChange={(e) => setSemantic(e.target.checked)}
              />
            }
            label={
              <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 0.5 }}>
                <AutoAwesomeIcon fontSize="small" />
                Semantic
              </Box>
            }
          />
        )}
        {(groupsQuery.data ?? []).map((g) => (
          <FormControl key={g.categoryGroupID} size="small" sx={{ minWidth: 140 }}>
            <InputLabel id={`cat-group-${g.categoryGroupID}`}>{g.groupName}</InputLabel>
            <Select
              labelId={`cat-group-${g.categoryGroupID}`}
              multiple
              value={categoryIds.filter((id) =>
                g.categories.some((c) => c.categoryID === id)
              )}
              onChange={(e) => {
                const picked = (e.target.value as number[]).map(Number);
                const groupIds = new Set(g.categories.map((c) => c.categoryID));
                setCategoryIds((prev) => [
                  ...prev.filter((id) => !groupIds.has(id)),
                  ...(g.exclusive ? picked.slice(-1) : picked),
                ]);
              }}
              input={<OutlinedInput label={g.groupName} />}
              renderValue={(selected) =>
                selected
                  .map((id) => g.categories.find((c) => c.categoryID === id)?.categoryName)
                  .filter(Boolean)
                  .join(", ")
              }
            >
              {g.categories.map((c) => (
                <MenuItem key={c.categoryID} value={c.categoryID}>
                  <Checkbox checked={categoryIds.includes(c.categoryID)} />
                  <ListItemText primary={c.categoryName} />
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        ))}
        {isAdmin && (
          <Button variant="outlined" onClick={handleUploadClick}>
            Upload Recipe Scan
          </Button>
        )}
        {canSuggestCocktails && (
          <Button
            variant="outlined"
            onClick={() => {
              setCocktailsOpen(true);
              cocktailMutation.mutate(inStockOnly);
            }}
          >
            Cocktail ideas
          </Button>
        )}
      </Box>
      {semanticIdle ? (
        <Alert severity="info" icon={<AutoAwesomeIcon />}>
          Describe what you&apos;re in the mood for — semantic search matches
          recipes by meaning, not just the name.
        </Alert>
      ) : (
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
      )}
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
            Choose File{" "}
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
      <Dialog open={cocktailsOpen} onClose={() => setCocktailsOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>Cocktail ideas</DialogTitle>
        <DialogContent>
          <FormControlLabel
            control={
              <Switch
                checked={inStockOnly}
                onChange={(e) => {
                  setInStockOnly(e.target.checked);
                  cocktailMutation.mutate(e.target.checked);
                }}
              />
            }
            label="Only what I can make from my pantry"
          />
          {cocktailMutation.isPending && <CircularProgress size={24} sx={{ mt: 1 }} />}
          {cocktailMutation.error && (
            <Alert severity="error" sx={{ mt: 1 }}>
              {(cocktailMutation.error as Error).message}
            </Alert>
          )}
          {cocktailMutation.data?.length === 0 && !cocktailMutation.isPending && (
            <DialogContentText sx={{ mt: 1 }}>
              No cocktail recipes in the catalog yet — tag some with the
              Cocktail dish type.
            </DialogContentText>
          )}
          {(cocktailMutation.data ?? []).map((c: CocktailSuggestion) => (
            <Box key={c.recipe.recipeID} sx={{ mt: 1.5 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Button
                  component={Link}
                  href={`/recipes/${c.recipe.recipeID}`}
                  size="small"
                >
                  {c.recipe.recipeName}
                </Button>
                {c.missingIngredients.length === 0 && (
                  <Chip size="small" color="success" label="In stock" />
                )}
              </Box>
              <DialogContentText variant="body2">{c.reason}</DialogContentText>
              {c.missingIngredients.length > 0 && (
                <DialogContentText variant="caption">
                  Missing: {c.missingIngredients.join(", ")}
                </DialogContentText>
              )}
            </Box>
          ))}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCocktailsOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
