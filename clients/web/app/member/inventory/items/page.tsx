"use client";

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Accordion from "@mui/material/Accordion";
import AccordionDetails from "@mui/material/AccordionDetails";
import AccordionSummary from "@mui/material/AccordionSummary";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import Autocomplete from "@mui/material/Autocomplete";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import Switch from "@mui/material/Switch";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import { api, asEntity } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import ConfirmDialog from "@/app/components/ConfirmDialog";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";
import QuantityDialog from "@/app/components/QuantityDialog";
import IngredientAutocomplete from "@/app/components/IngredientAutocomplete";
import AllergenFlagsEditor from "@/app/components/AllergenFlagsEditor";
import { AllergenFlag, Item, Brand, Ingredient } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";

// The generated fields cover the item's identity plus the pantry holding.
// Catalog extras (barcodes, brand, category, ingredient links) render as
// pickers/fields in extraFields inside a collapsed "Catalog details"
// section — the dialog's everyday flow stays short.
const itemFields = [
  { key: "name", label: "Name" },
  { key: "unit", label: "Unit" },
  { key: "currentQuantity", label: "Current Quantity", type: "number" as const },
  { key: "minQuantity", label: "Min Quantity", type: "number" as const },
  { key: "purchaseDate", label: "Purchase Date", type: "date" as const },
  { key: "expiryDate", label: "Expiry Date", type: "date" as const },
  { key: "notes", label: "Notes" },
];

export default function ItemsPage() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogData, setDialogData] = useState<Record<string, unknown>>({});
  const [isCreate, setIsCreate] = useState(false);
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [brandId, setBrandId] = useState<number | null>(null);
  const [brandInput, setBrandInput] = useState("");
  const [debouncedBrandInput, setDebouncedBrandInput] = useState("");
  const [inStock, setInStock] = useState(false);
  const [isFavorite, setIsFavorite] = useState(false);
  // Server-side sort: only "name" is in the items allowlist — every other
  // column is marked sortable:false below so no dead affordance shows.
  const [sort, setSort] = useState<{ key: string; desc: boolean } | null>(null);
  const [qtyDialogItem, setQtyDialogItem] = useState<Item | null>(null);
  const [qtyEditItem, setQtyEditItem] = useState<Item | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Item | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveNotice, setSaveNotice] = useState<string | null>(null);
  const [catDialogItem, setCatDialogItem] = useState<Item | null>(null);
  // undefined = untouched; null = clear the link; Brand/Ingredient = set it.
  const [dialogBrand, setDialogBrand] = useState<Brand | null | undefined>(undefined);
  const [dialogBrandInput, setDialogBrandInput] = useState("");
  const [dialogCategoryId, setDialogCategoryId] = useState<number | "">("");
  const [dialogCatalogIng, setDialogCatalogIng] = useState<Ingredient | null | undefined>(undefined);
  const [dialogHouseholdIng, setDialogHouseholdIng] = useState<Ingredient | null | undefined>(undefined);
  // Barcode fields live in the collapsed catalog section — controlled here
  // (like dialogBrand) and merged into the save payload in handleSave.
  const [dialogUpc12, setDialogUpc12] = useState("");
  const [dialogUpc14, setDialogUpc14] = useState("");
  const { me } = useMe();
  const isAdmin = me?.role === "admin";

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedBrandInput(brandInput), 300);
    return () => clearTimeout(timer);
  }, [brandInput]);

  useEffect(() => {
    if (debouncedSearch.trim()) {
      void api.recordSearch("item", debouncedSearch);
    }
  }, [debouncedSearch]);

  useEffect(() => {
    if (debouncedBrandInput.trim()) {
      void api.recordSearch("brand", debouncedBrandInput);
    }
  }, [debouncedBrandInput]);

  useEffect(() => {
    // eslint-disable-next-line @eslint-react/set-state-in-effect
    setPageNumber(1);
  }, [debouncedSearch, brandId, inStock, isFavorite]);

  const brandsQuery = useQuery({
    queryKey: ["item-brands", brandInput],
    queryFn: () =>
      brandInput === ""
        ? api.getFrequentBrands(10)
        : api.getBrands(brandInput),
  });

  // The edit dialog gets its own brand search so typing there doesn't
  // disturb the toolbar filter.
  const dialogBrandsQuery = useQuery({
    queryKey: ["item-dialog-brands", dialogBrandInput],
    queryFn: () =>
      dialogBrandInput === ""
        ? api.getFrequentBrands(10)
        : api.getBrands(dialogBrandInput),
    enabled: dialogOpen,
  });

  const dialogCategoriesQuery = useQuery({
    queryKey: ["item-dialog-categories"],
    queryFn: () => api.getActiveCategories(),
    enabled: dialogOpen || catDialogItem !== null,
  });

  const brandOptions = brandsQuery.data ?? [];
  const selectedBrand =
    brandId === null
      ? null
      : brandOptions.find((b) => b.brandID === brandId) ?? null;

  const listQuery = useQuery({
    queryKey: ["items", pageNumber, pageSize, debouncedSearch, brandId, inStock, isFavorite, sort],
    queryFn: () => api.getItemsPaged(pageNumber, pageSize, debouncedSearch, brandId ?? undefined, inStock, isFavorite, sort ?? undefined),
    placeholderData: (prev) => prev,
  });

  const createMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) => api.createItem(asEntity(row)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["items"] }),
  });

  const updateMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.updateItem(row.itemID as number, asEntity(row)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["items"] }),
  });

  const deleteMutation = useMutation({
    mutationFn: (row: Item) => api.deleteItem(row.itemID),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["items"] }),
  });

  const changeCategoryMutation = useMutation({
    mutationFn: ({ id, categoryId }: { id: number; categoryId: number }) =>
      api.changeItemCategory(id, categoryId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["items"] }),
  });

  const adjustQuantityMutation = useMutation({
    mutationFn: ({
      id,
      quantity,
      purchaseDate,
    }: {
      id: number;
      quantity: number;
      purchaseDate?: string;
    }) => api.adjustItemQuantity(id, quantity, purchaseDate),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["items"] }),
  });

  const setFavoriteMutation = useMutation({
    mutationFn: ({ id, isFavorite }: { id: number; isFavorite: boolean }) =>
      api.setItemFavorite(id, isFavorite),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["items"] }),
  });

  const handleCreate = () => {
    createMutation.reset();
    updateMutation.reset();
    setSaveNotice(null);
    setIsCreate(true);
    setDialogData({});
    setDialogBrand(undefined);
    setDialogBrandInput("");
    setDialogCategoryId("");
    setDialogCatalogIng(undefined);
    setDialogHouseholdIng(undefined);
    setDialogUpc12("");
    setDialogUpc14("");
    setDialogOpen(true);
  };

  const handleEdit = (row: Item) => {
    void api.recordView("item", row.itemID);
    createMutation.reset();
    updateMutation.reset();
    setSaveNotice(null);
    setIsCreate(false);
    setDialogData({ ...row });
    setDialogBrand(undefined);
    setDialogBrandInput(row.brand ?? "");
    setDialogCategoryId(row.categoryID);
    setDialogCatalogIng(undefined);
    setDialogHouseholdIng(undefined);
    setDialogUpc12(row.upc12 ?? "");
    setDialogUpc14(row.upc14 ?? "");
    setDialogOpen(true);
  };

  const handleDelete = (row: Item) => {
    setDeleteTarget(row);
  };

  const applyIngredientLinks = async (itemID: number, failed: string[]) => {
    const jobs: Promise<void>[] = [];
    if (dialogCatalogIng !== undefined && isAdmin) {
      jobs.push(api.setItemIngredient(itemID, dialogCatalogIng?.ingredientID ?? null));
    }
    if (dialogHouseholdIng !== undefined) {
      jobs.push(api.setHouseholdItemIngredient(itemID, dialogHouseholdIng?.ingredientID ?? null));
    }
    if (jobs.length === 0) return;
    try {
      await Promise.all(jobs);
      queryClient.invalidateQueries({ queryKey: ["items"] });
    } catch {
      failed.push("ingredient links");
    }
  };

  const strOrNull = (v: unknown): string | null =>
    typeof v === "string" && v !== "" ? v : null;
  const numOrNull = (v: unknown): number | null =>
    v === undefined || v === null || v === "" ? null : Number(v);

  // Pantry fields live on the household holding, not the catalog row —
  // they round-trip through adjustUserItem. Empty dialog inputs serialize
  // as null, which clears the stored value server-side (explicit clear).
  const pantryFieldsChanged = (
    values: Record<string, unknown>,
    prev: Record<string, unknown>
  ) =>
    Number(values.currentQuantity ?? 0) !== Number(prev.currentQuantity ?? 0) ||
    strOrNull(values.expiryDate) !== strOrNull(prev.expiryDate) ||
    numOrNull(values.minQuantity) !== numOrNull(prev.minQuantity) ||
    strOrNull(values.purchaseDate) !== strOrNull(prev.purchaseDate) ||
    strOrNull(values.notes) !== strOrNull(prev.notes);

  const handleSave = async (values: Record<string, unknown>) => {
    if (dialogBrand !== undefined) {
      values.brandID = dialogBrand?.brandID ?? null;
    }
    if (dialogCategoryId !== "") {
      values.categoryID = dialogCategoryId;
    }
    // Barcode fields are controlled (catalog section), not generated —
    // empty input serializes as null, which clears the stored value.
    values.upc12 = strOrNull(dialogUpc12.trim());
    values.upc14 = strOrNull(dialogUpc14.trim());
    const failed: string[] = [];
    setSaving(true);
    // Pantry holding write shared by create and edit; empty inputs clear
    // server-side, so "should we write" is about whether anything was set.
    const adjustPantry = async (itemID: number) => {
      const qty = Number(values.currentQuantity ?? 0);
      if (
        qty > 0 ||
        strOrNull(values.expiryDate) ||
        numOrNull(values.minQuantity) !== null ||
        strOrNull(values.purchaseDate) ||
        strOrNull(values.notes)
      ) {
        await api.adjustItemQuantity(
          itemID,
          qty,
          strOrNull(values.purchaseDate),
          strOrNull(values.expiryDate),
          numOrNull(values.minQuantity),
          strOrNull(values.notes)
        );
        queryClient.invalidateQueries({ queryKey: ["items"] });
      }
    };
    const run = async (label: string, job: () => Promise<unknown>) => {
      try {
        await job();
      } catch {
        failed.push(label);
      }
    };
    try {
      if (isCreate) {
        const created = await createMutation.mutateAsync(values);
        await applyIngredientLinks(created.itemID, failed);
        await run("pantry details", () => adjustPantry(created.itemID));
      } else {
        const itemID = values.itemID as number;
        await updateMutation.mutateAsync(values);
        await applyIngredientLinks(itemID, failed);
        if (pantryFieldsChanged(values, dialogData)) {
          await run("pantry details", () => adjustPantry(itemID));
        }
      }
    } catch {
      // Main mutation failed — its error state is shown in the dialog.
      setSaving(false);
      return;
    }
    setSaving(false);
    if (failed.length > 0) {
      setSaveNotice(`Item saved, but ${failed.join(", ")} didn't apply.`);
    }
    setDialogOpen(false);
  };

  const handleChangeCategory = (row: Item) => {
    setCatDialogItem(row);
  };

  const handleSaveCategory = (values: Record<string, unknown>) => {
    if (!catDialogItem || values.categoryID === "") return;
    changeCategoryMutation.mutate(
      { id: catDialogItem.itemID, categoryId: Number(values.categoryID) },
      { onSuccess: () => setCatDialogItem(null) }
    );
  };

  const handleSaveQuantity = (quantity: number) => {
    if (!qtyEditItem) return;
    adjustQuantityMutation.mutate({ id: qtyEditItem.itemID, quantity });
    setQtyEditItem(null);
  };

  const handleAddToInventory = (quantity: number) => {
    if (!qtyDialogItem) return;
    adjustQuantityMutation.mutate({
      id: qtyDialogItem.itemID,
      quantity: qtyDialogItem.currentQuantity + quantity,
    });
    setQtyDialogItem(null);
  };

  // Only "name" is server-sortable; brand/ingredient are joins and the
  // pantry columns come from the per-row userItems overlay.
  const itemColumns: FieldDef<Item>[] = [
    { key: "name", label: "Name", minWidth: 220, sortable: true },
    { key: "brand", label: "Brand", sortable: false },
    {
      key: "ingredient",
      label: "Ingredient",
      sortable: false,
      render: (row) => row.householdIngredient?.name ?? row.ingredient?.name ?? "—",
    },
    {
      key: "currentQuantity",
      label: "Qty",
      type: "number",
      sortable: false,
      render: (row) => (
        <Chip
          size="small"
          variant="outlined"
          clickable
          label={`${row.currentQuantity} ${row.unit}`}
          onClick={() => setQtyEditItem(row)}
          title="Adjust quantity"
        />
      ),
    },
    { key: "minQuantity", label: "Min Qty", type: "number", sortable: false },
    { key: "purchaseDate", label: "Purchase Date", sortable: false },
    { key: "expiryDate", label: "Expiry Date", sortable: false },
    { key: "notes", label: "Notes", sortable: false },
    {
      key: "isFavorite",
      label: "Favorite",
      sortable: false,
      render: (row) => (
        <Chip
          size="small"
          variant="outlined"
          clickable
          label={row.isFavorite ? "Yes" : "No"}
          onClick={() =>
            setFavoriteMutation.mutate({
              id: row.itemID,
              isFavorite: !row.isFavorite,
            })
          }
          title="Toggle favorite"
        />
      ),
    },
  ];

  const extraActions = (row: Item) => (
    <Box sx={{ display: "flex", gap: 0.5, flexWrap: "wrap" }}>
      <Button
        size="small"
        onClick={() => setQtyDialogItem(row)}
      >
        Add to Inventory
      </Button>
      <Button
        size="small"
        onClick={() => handleChangeCategory(row)}
      >
        Category
      </Button>
    </Box>
  );

  return (
    <Box>
      <Box
        sx={{
          display: "flex",
          gap: 2,
          flexWrap: "wrap",
          mb: 2,
        }}
      >
        <TextField
          size="small"
          label="Search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          sx={{ minWidth: 200 }}
        />
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
        <FormControlLabel
          control={
            <Switch
              checked={inStock}
              onChange={(e) => setInStock(e.target.checked)}
            />
          }
          label="In Stock"
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
      {saveNotice && (
        <Alert severity="warning" onClose={() => setSaveNotice(null)} sx={{ mb: 2 }}>
          {saveNotice}
        </Alert>
      )}
      <DataTable
        title="Items"
        rows={listQuery.data?.items ?? []}
        isLoading={listQuery.isLoading}
        error={(listQuery.error ?? deleteMutation.error) as Error | null}
        onCreate={handleCreate}
        onEdit={handleEdit}
        onDelete={handleDelete}
        extraActions={extraActions}
        fields={itemColumns}
        serverSort={(key, desc) => {
          setSort({ key, desc });
          setPageNumber(1);
        }}
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
        title={isCreate ? "Create Item" : "Edit Item"}
        fields={itemFields}
        values={dialogData}
        error={createMutation.error ?? updateMutation.error}
        saving={saving}
        extraFields={
          // Catalog attributes are rarely touched — collapse them so the
          // everyday pantry fields stay above the fold.
          <Accordion sx={{ mt: 1, boxShadow: "none" }} disableGutters>
            <AccordionSummary
              expandIcon={<ExpandMoreIcon />}
              aria-controls="item-catalog-details"
              id="item-catalog-details-header"
            >
              <Typography color="text.secondary">
                Catalog details — barcodes, brand, category, ingredients
              </Typography>
            </AccordionSummary>
            <AccordionDetails>
            <TextField
              label="UPC-A"
              helperText="12-digit barcode (US products)"
              size="small"
              fullWidth
              margin="dense"
              value={dialogUpc12}
              onChange={(e) => setDialogUpc12(e.target.value)}
            />
            <TextField
              label="GTIN-14"
              helperText="14-digit barcode (international / case packs)"
              size="small"
              fullWidth
              margin="dense"
              value={dialogUpc14}
              onChange={(e) => setDialogUpc14(e.target.value)}
            />
            <FormControl size="small" fullWidth sx={{ mt: 1 }}>
              <InputLabel id="item-category-label">Category</InputLabel>
              <Select
                labelId="item-category-label"
                label="Category"
                value={dialogCategoryId}
                onChange={(e) =>
                  setDialogCategoryId(Number(e.target.value))
                }
              >
                {(dialogCategoriesQuery.data ?? []).map((c) => (
                  <MenuItem key={c.categoryID} value={c.categoryID}>
                    {c.categoryName}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <Autocomplete
              size="small"
              options={dialogBrandsQuery.data ?? []}
              getOptionLabel={(b) => (typeof b === "string" ? b : b?.brandName ?? "")}
              isOptionEqualToValue={(a, b) =>
                (typeof a === "object" && typeof b === "object" && a?.brandID === b?.brandID)
              }
              inputValue={dialogBrandInput}
              onInputChange={(_, value) => setDialogBrandInput(value)}
              value={
                dialogBrand !== undefined
                  ? dialogBrand
                  : (dialogBrandsQuery.data ?? []).find(
                      (b) => b.brandID === dialogData.brandID
                    ) ?? null
              }
              onChange={(_, value) => setDialogBrand((value as Brand | null) ?? null)}
              filterOptions={(options) => options}
              loading={dialogBrandsQuery.isLoading}
              noOptionsText="No brands found"
              renderInput={(params) => (
                <TextField {...params} label="Brand" size="small" margin="dense" />
              )}
            />
            {isAdmin && (
              <IngredientAutocomplete
                label="Catalog ingredient"
                helperText="The shared ingredient this item represents (admin-only link)"
                size="medium"
                value={
                  dialogCatalogIng !== undefined
                    ? dialogCatalogIng
                    : ((dialogData.ingredient as Ingredient | null) ?? null)
                }
                onChange={setDialogCatalogIng}
              />
            )}
            <IngredientAutocomplete
              label="Household ingredient"
              helperText={
                isAdmin
                  ? "Override for your household only"
                  : "The generic ingredient this item is for your household"
              }
              size="medium"
              value={
                dialogHouseholdIng !== undefined
                  ? dialogHouseholdIng
                  : ((dialogData.householdIngredient as Ingredient | null) ?? null)
              }
              onChange={setDialogHouseholdIng}
            />
            {isAdmin && (
              <AllergenFlagsEditor
                key={isCreate ? "new" : (dialogData.itemID as number)}
                flags={dialogData.allergens as AllergenFlag[] | undefined}
                disabledReason={
                  isCreate ? "Save the item first to set allergen flags." : undefined
                }
                onSet={async (allergenID, kind) => {
                  await api.setItemAllergen(
                    dialogData.itemID as number,
                    allergenID,
                    kind
                  );
                  void queryClient.invalidateQueries({ queryKey: ["items"] });
                }}
              />
            )}
            </AccordionDetails>
          </Accordion>
        }
        onClose={() => setDialogOpen(false)}
        onSave={handleSave}
      />
      <QuantityDialog
        open={qtyDialogItem !== null}
        title={
          qtyDialogItem
            ? `Add ${qtyDialogItem.name} to Inventory`
            : "Add to Inventory"
        }
        label="How many to add"
        onClose={() => setQtyDialogItem(null)}
        onConfirm={handleAddToInventory}
      />
      <QuantityDialog
        open={qtyEditItem !== null}
        title={
          qtyEditItem ? `Edit ${qtyEditItem.name} Quantity` : "Edit Quantity"
        }
        label="Quantity"
        confirmLabel="Save"
        min={0}
        initialValue={qtyEditItem?.currentQuantity}
        onClose={() => setQtyEditItem(null)}
        onConfirm={handleSaveQuantity}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete item"
        message={
          deleteTarget
            ? `Delete "${deleteTarget.name}"? Its pantry record and settings go with it.`
            : ""
        }
        confirmLabel="Delete"
        destructive
        busy={deleteMutation.isPending}
        onConfirm={() => {
          const row = deleteTarget;
          setDeleteTarget(null);
          if (row) deleteMutation.mutate(row);
        }}
        onClose={() => setDeleteTarget(null)}
      />
      <CrudDialog
        open={catDialogItem !== null}
        title={
          catDialogItem
            ? `Change ${catDialogItem.name} Category`
            : "Change Category"
        }
        fields={[
          {
            key: "categoryID",
            label: "Category",
            type: "select",
            options: (dialogCategoriesQuery.data ?? []).map((c) => ({
              value: c.categoryID,
              label: c.categoryName,
            })),
          },
        ]}
        values={{ categoryID: catDialogItem?.categoryID ?? "" }}
        error={changeCategoryMutation.error}
        saving={changeCategoryMutation.isPending}
        onClose={() => setCatDialogItem(null)}
        onSave={handleSaveCategory}
      />
    </Box>
  );
}
