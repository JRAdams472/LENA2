"use client";

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Autocomplete from "@mui/material/Autocomplete";
import TextField from "@mui/material/TextField";
import Switch from "@mui/material/Switch";
import FormControlLabel from "@mui/material/FormControlLabel";
import { api, asEntity } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import CrudDialog from "@/app/components/CrudDialog";
import { Item, Brand } from "@/lib/types";

const itemFields = [
  { key: "name", label: "Name" },
  { key: "brand", label: "Brand" },
  { key: "upc12", label: "UPC12" },
  { key: "upc14", label: "UPC14" },
  { key: "categoryID", label: "Category ID", type: "number" as const },
  { key: "unit", label: "Unit" },
  { key: "currentQuantity", label: "Current Quantity", type: "number" as const },
  { key: "minQuantity", label: "Min Quantity", type: "number" as const },
  { key: "purchaseDate", label: "Purchase Date" },
  { key: "expiryDate", label: "Expiry Date" },
  { key: "notes", label: "Notes" },
  { key: "isFavorite", label: "Favorite", type: "boolean" as const },
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
  const [brand, setBrand] = useState<string | "">("");
  const [brandInput, setBrandInput] = useState("");
  const [inStock, setInStock] = useState(false);
  const [isFavorite, setIsFavorite] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPageNumber(1);
  }, [debouncedSearch, brand, inStock, isFavorite]);

  const brandsQuery = useQuery({
    queryKey: ["item-brands", brandInput],
    queryFn: () =>
      brandInput === ""
        ? api.getFrequentBrands(10)
        : api.getBrands(brandInput),
  });

  const brandOptions = brandsQuery.data ?? [];
  const selectedBrand =
    brand === "" ? null : brandOptions.find((b) => b.brandName === brand) ?? null;

  const listQuery = useQuery({
    queryKey: ["items", pageNumber, pageSize, debouncedSearch, brand, inStock, isFavorite],
    queryFn: () => api.getItemsPaged(pageNumber, pageSize, debouncedSearch, brand, inStock, isFavorite),
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
    setIsCreate(true);
    setDialogData({});
    setDialogOpen(true);
  };

  const handleEdit = (row: Item) => {
    setIsCreate(false);
    setDialogData({ ...row });
    setDialogOpen(true);
  };

  const handleDelete = (row: Item) => {
    if (window.confirm("Delete this item?")) {
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

  const handleChangeCategory = (id: number) => {
    const value = window.prompt("Enter new Category ID");
    if (value === null) return;
    const categoryId = Number(value);
    if (isNaN(categoryId)) {
      alert("Category ID must be a number");
      return;
    }
    changeCategoryMutation.mutate({ id, categoryId });
  };

  const handleAdjustQuantity = (id: number) => {
    const value = window.prompt("Enter quantity adjustment");
    if (value === null) return;
    const quantity = Number(value);
    if (isNaN(quantity)) {
      alert("Quantity must be a number");
      return;
    }
    const purchaseDate = window.prompt(
      "Enter purchase date (ISO, optional)"
    );
    adjustQuantityMutation.mutate({
      id,
      quantity,
      purchaseDate: purchaseDate || undefined,
    });
  };

  const handleToggleFavorite = (id: number, current: boolean) => {
    setFavoriteMutation.mutate({ id, isFavorite: !current });
  };

  const extraActions = (row: Item) => (
    <Box sx={{ display: "flex", gap: 0.5, flexWrap: "wrap" }}>
      <Button
        size="small"
        onClick={() => handleChangeCategory(row.itemID)}
      >
        Category
      </Button>
      <Button
        size="small"
        onClick={() => handleAdjustQuantity(row.itemID)}
      >
        Qty
      </Button>
      <Button
        size="small"
        onClick={() =>
          handleToggleFavorite(row.itemID, row.isFavorite)
        }
      >
        {row.isFavorite ? "Unfav" : "Fav"}
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
            setBrand(b?.brandName ?? "");
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
      <DataTable
        title="Items"
        rows={listQuery.data?.items ?? []}
        isLoading={listQuery.isLoading}
        error={listQuery.error as Error | null}
        onCreate={handleCreate}
        onEdit={handleEdit}
        onDelete={handleDelete}
        extraActions={extraActions}
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
        onClose={() => setDialogOpen(false)}
        onSave={handleSave}
      />
    </Box>
  );
}
