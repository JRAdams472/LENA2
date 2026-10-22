"use client";

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Autocomplete from "@mui/material/Autocomplete";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { api, asEntity } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";
import QuantityDialog from "@/app/components/QuantityDialog";
import { Bottle, PagedResult } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";

// Catalog attributes — admin-only (createBottle/updateBottle are @admin).
// The FKs (type/country/region/vintage) render as pickers in extraFields.
const catalogFields = [
  { key: "vineyard", label: "Vineyard" },
  { key: "abv", label: "ABV", type: "number" as const },
  { key: "acidity", label: "Acidity", type: "number" as const },
  { key: "tanninLevel", label: "Tannin Level", type: "number" as const },
  { key: "body", label: "Body", type: "number" as const },
  { key: "sweetness", label: "Sweetness", type: "number" as const },
  { key: "oakIntegration", label: "Oak Integration", type: "boolean" as const },
  { key: "bottleSize", label: "Bottle Size" },
];

// Cellar fields — the household's holding, written via updateUserBottle.
// Empty values serialize as null, which clears the field server-side
// (quantity excepted: null is rejected, so empty stays untouched).
const cellarFields = [
  { key: "quantity", label: "Quantity", type: "number" as const },
  { key: "bottleNumber", label: "Bottle Number", type: "number" as const },
  { key: "purchaseDate", label: "Purchase Date" },
  { key: "purchasePrice", label: "Purchase Price", type: "number" as const },
  { key: "storageTemp", label: "Storage Temp", type: "number" as const },
  { key: "location", label: "Location" },
  { key: "notes", label: "Notes" },
  { key: "isFavorite", label: "Favorite", type: "boolean" as const },
];

const numOrUndef = (v: unknown): number | undefined =>
  v === undefined || v === null || v === "" ? undefined : Number(v);
const numOrNull = (v: unknown): number | null => numOrUndef(v) ?? null;
const strOrNull = (v: unknown): string | null =>
  typeof v === "string" && v !== "" ? v : null;

export default function BottlesPage() {
  const queryClient = useQueryClient();
  const [countryId, setCountryId] = useState<string>("");
  const [regionId, setRegionId] = useState<string>("");
  const [typeId, setTypeId] = useState<string>("");
  const [vintageYear, setVintageYear] = useState<string>("");
  const [favorites, setFavorites] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(25);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchTerm);
      setPageNumber(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchTerm]);

  useEffect(() => {
    if (debouncedSearch.trim()) {
      void api.recordSearch("bottle", debouncedSearch);
    }
  }, [debouncedSearch]);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogData, setDialogData] = useState<Record<string, unknown>>({});
  const [isCreate, setIsCreate] = useState(false);
  const [qtyBottle, setQtyBottle] = useState<Bottle | null>(null);
  // FK picker + add-to-cellar state (lives outside the generated fields).
  const [pickTypeId, setPickTypeId] = useState<number | "">("");
  const [pickCountryId, setPickCountryId] = useState<number | "">("");
  const [pickRegionId, setPickRegionId] = useState<number | "">("");
  const [pickVintageYear, setPickVintageYear] = useState<number | "">("");
  const [cellarBottle, setCellarBottle] = useState<Bottle | null>(null);
  const [cellarSearch, setCellarSearch] = useState("");
  const { me } = useMe();
  const isAdmin = me?.role === "admin";

  const countriesQuery = useQuery({
    queryKey: ["countries"],
    queryFn: () => api.getCountries(),
  });
  const regionsQuery = useQuery({
    queryKey: ["regions"],
    queryFn: () => api.getRegions(),
  });
  const typesQuery = useQuery({
    queryKey: ["types"],
    queryFn: () => api.getTypes(),
  });
  const vintagesQuery = useQuery({
    queryKey: ["vintages"],
    queryFn: () => api.getVintages(),
  });
  const cellarPickQuery = useQuery({
    queryKey: ["cellar-bottle-pick", cellarSearch],
    queryFn: () => api.searchBottles(cellarSearch),
    enabled: dialogOpen && isCreate && !isAdmin,
  });

  const listQuery = useQuery<PagedResult<Bottle>>({
    queryKey: [
      "bottles",
      countryId,
      regionId,
      typeId,
      vintageYear,
      favorites,
      debouncedSearch,
      pageNumber,
      pageSize,
    ],
    queryFn: () =>
      api.getBottlesPaged(pageNumber, pageSize, {
        search: debouncedSearch,
        countryId: countryId ? Number(countryId) : undefined,
        regionId: regionId ? Number(regionId) : undefined,
        typeId: typeId ? Number(typeId) : undefined,
        vintageYear: vintageYear ? Number(vintageYear) : undefined,
        favoritesOnly: favorites || undefined,
      }),
    placeholderData: (prev) => prev,
  });

  const countQuery = useQuery({
    queryKey: ["bottle-count"],
    queryFn: api.getBottleCount,
  });

  const invalidateBottles = () =>
    queryClient.invalidateQueries({ queryKey: ["bottles"] });

  const saveMutation = useMutation({
    mutationFn: async ({
      values,
      create,
    }: {
      values: Record<string, unknown>;
      create: boolean;
    }) => {
      const cellarInput = {
        quantity: numOrUndef(values.quantity),
        bottleNumber: numOrNull(values.bottleNumber),
        purchaseDate: strOrNull(values.purchaseDate),
        purchasePrice: numOrNull(values.purchasePrice),
        storageTemp: numOrNull(values.storageTemp),
        location: strOrNull(values.location),
        notes: strOrNull(values.notes),
      };
      const jobs: Promise<unknown>[] = [];

      let bottleID = create ? 0 : (values.bottleID as number);
      if (create && isAdmin) {
        const created = await api.createBottle(
          asEntity({
            ...values,
            typeID: pickTypeId === "" ? 0 : pickTypeId,
            countryID: pickCountryId === "" ? 0 : pickCountryId,
            regionID: pickRegionId === "" ? 0 : pickRegionId,
            vintageYear: pickVintageYear === "" ? 0 : pickVintageYear,
          })
        );
        bottleID = created.bottleID;
      } else if (create) {
        bottleID = cellarBottle?.bottleID ?? 0;
        if (!bottleID) throw new Error("Pick a bottle to add to the cellar");
      } else if (isAdmin) {
        jobs.push(
          api.updateBottle(bottleID, {
            ...asEntity(values),
            typeID: pickTypeId === "" ? undefined : pickTypeId,
            countryID: pickCountryId === "" ? undefined : pickCountryId,
            regionID: pickRegionId === "" ? undefined : pickRegionId,
            vintageYear: pickVintageYear === "" ? undefined : pickVintageYear,
          })
        );
      }

      jobs.push(api.updateUserBottle(bottleID, cellarInput));
      if (Boolean(values.isFavorite) !== Boolean(dialogData.isFavorite)) {
        jobs.push(api.setBottleFavorite(bottleID, Boolean(values.isFavorite)));
      }
      await Promise.all(jobs);
    },
    onSuccess: invalidateBottles,
  });

  const adjustQtyMutation = useMutation({
    mutationFn: ({ id, quantity }: { id: number; quantity: number }) =>
      api.adjustBottleQuantity(id, quantity),
    onSuccess: invalidateBottles,
  });

  const setFavoriteMutation = useMutation({
    mutationFn: ({ id, isFavorite }: { id: number; isFavorite: boolean }) =>
      api.setBottleFavorite(id, isFavorite),
    onSuccess: invalidateBottles,
  });

  const deleteMutation = useMutation({
    mutationFn: (row: Bottle) => api.deleteBottle(row.bottleID),
    onSuccess: invalidateBottles,
  });

  const handleCreate = () => {
    setIsCreate(true);
    setDialogData({});
    setPickTypeId("");
    setPickCountryId("");
    setPickRegionId("");
    setPickVintageYear("");
    setCellarBottle(null);
    setDialogOpen(true);
  };

  const handleEdit = (row: Bottle) => {
    void api.recordView("bottle", row.bottleID);
    setIsCreate(false);
    setDialogData({ ...row });
    setPickTypeId(row.typeID);
    setPickCountryId(row.countryID);
    setPickRegionId(row.regionID);
    setPickVintageYear(row.vintageYear);
    setDialogOpen(true);
  };

  const handleDelete = (row: Bottle) => {
    if (window.confirm("Delete this bottle?")) {
      deleteMutation.mutate(row);
    }
  };

  const handleSave = (values: Record<string, unknown>) => {
    saveMutation.mutate(
      { values, create: isCreate },
      { onSuccess: () => setDialogOpen(false) }
    );
  };

  const bottleTableFields: FieldDef<Bottle>[] = [
    { key: "vineyard", label: "Vineyard", render: (r) => r.vineyard ?? "—" },
    { key: "vintageYear", label: "Vintage", type: "number" },
    { key: "type", label: "Type", render: (r) => r.type?.typeName ?? "—" },
    { key: "country", label: "Country", render: (r) => r.country?.countryName ?? "—" },
    { key: "region", label: "Region", render: (r) => r.region?.regionName ?? "—" },
    { key: "bottleSize", label: "Size" },
    {
      key: "quantity",
      label: "Qty",
      render: (r) => (
        <Chip
          size="small"
          variant="outlined"
          clickable
          label={r.quantity}
          onClick={() => setQtyBottle(r)}
          title="Set cellar quantity"
        />
      ),
    },
    { key: "location", label: "Location", render: (r) => r.location ?? "—" },
    {
      key: "isFavorite",
      label: "Favorite",
      render: (r) => (
        <Chip
          size="small"
          variant="outlined"
          clickable
          label={r.isFavorite ? "Yes" : "No"}
          onClick={() =>
            setFavoriteMutation.mutate({ id: r.bottleID, isFavorite: !r.isFavorite })
          }
          title="Toggle favorite"
        />
      ),
    },
  ];

  const countries = countriesQuery.data ?? [];
  const regions = regionsQuery.data ?? [];
  const types = typesQuery.data ?? [];
  const vintages = vintagesQuery.data ?? [];

  // Regions are filtered by the picked country when one is set.
  const pickRegions =
    pickCountryId === ""
      ? regions
      : regions.filter((r) => r.countryID === pickCountryId);

  const catalogPickers = isAdmin && (
    <>
      <FormControl size="small" fullWidth sx={{ mt: 1 }}>
        <InputLabel id="bottle-type-label">Type</InputLabel>
        <Select
          labelId="bottle-type-label"
          label="Type"
          value={pickTypeId}
          onChange={(e) => setPickTypeId(Number(e.target.value))}
        >
          {types.map((t) => (
            <MenuItem key={t.typeID} value={t.typeID}>
              {t.typeName}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <FormControl size="small" fullWidth sx={{ mt: 1 }}>
        <InputLabel id="bottle-country-label">Country</InputLabel>
        <Select
          labelId="bottle-country-label"
          label="Country"
          value={pickCountryId}
          onChange={(e) => {
            setPickCountryId(Number(e.target.value));
            setPickRegionId("");
          }}
        >
          {countries.map((c) => (
            <MenuItem key={c.countryID} value={c.countryID}>
              {c.countryName}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <FormControl size="small" fullWidth sx={{ mt: 1 }}>
        <InputLabel id="bottle-region-label">Region</InputLabel>
        <Select
          labelId="bottle-region-label"
          label="Region"
          value={pickRegionId}
          onChange={(e) => setPickRegionId(Number(e.target.value))}
        >
          {pickRegions.map((r) => (
            <MenuItem key={r.regionID} value={r.regionID}>
              {r.regionName}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <FormControl size="small" fullWidth sx={{ mt: 1 }}>
        <InputLabel id="bottle-vintage-label">Vintage</InputLabel>
        <Select
          labelId="bottle-vintage-label"
          label="Vintage"
          value={pickVintageYear}
          onChange={(e) => setPickVintageYear(Number(e.target.value))}
        >
          {vintages.map((v) => (
            <MenuItem key={v.vintageID} value={v.year}>
              {v.year}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    </>
  );

  const cellarPicker = isCreate && !isAdmin && (
    <Autocomplete
      options={cellarPickQuery.data ?? []}
      getOptionLabel={(b) => `${b.vineyard ?? "Bottle"} ${b.vintageYear}`}
      isOptionEqualToValue={(a, b) => a.bottleID === b.bottleID}
      inputValue={cellarSearch}
      onInputChange={(_, v) => setCellarSearch(v)}
      value={cellarBottle}
      onChange={(_, v) => setCellarBottle(v)}
      loading={cellarPickQuery.isLoading}
      noOptionsText="No catalog bottles found"
      renderInput={(params) => (
        <TextField {...params} label="Bottle" size="small" />
      )}
      sx={{ mt: 1 }}
    />
  );

  const pagedData = listQuery.data;
  const rows = pagedData?.items ?? [];

  return (
    <Box>
      <Box
        sx={{
          display: "flex",
          flexWrap: "wrap",
          gap: 2,
          alignItems: "center",
          mb: 2,
        }}
      >
        <FormControl sx={{ minWidth: 140 }}>
          <InputLabel id="filter-country-label">Country</InputLabel>
          <Select
            labelId="filter-country-label"
            value={countryId}
            onChange={(e) => { setCountryId(e.target.value); setPageNumber(1); }}
            label="Country"
          >
            <MenuItem value="">All</MenuItem>
            {countries.map((c) => (
              <MenuItem key={c.countryID} value={String(c.countryID)}>
                {c.countryName}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl sx={{ minWidth: 140 }}>
          <InputLabel id="filter-region-label">Region</InputLabel>
          <Select
            labelId="filter-region-label"
            value={regionId}
            onChange={(e) => { setRegionId(e.target.value); setPageNumber(1); }}
            label="Region"
          >
            <MenuItem value="">All</MenuItem>
            {regions.map((r) => (
              <MenuItem key={r.regionID} value={String(r.regionID)}>
                {r.regionName}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl sx={{ minWidth: 140 }}>
          <InputLabel id="filter-type-label">Type</InputLabel>
          <Select
            labelId="filter-type-label"
            value={typeId}
            onChange={(e) => { setTypeId(e.target.value); setPageNumber(1); }}
            label="Type"
          >
            <MenuItem value="">All</MenuItem>
            {types.map((t) => (
              <MenuItem key={t.typeID} value={String(t.typeID)}>
                {t.typeName}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl sx={{ minWidth: 120 }}>
          <InputLabel id="filter-vintage-label">Vintage</InputLabel>
          <Select
            labelId="filter-vintage-label"
            value={vintageYear}
            onChange={(e) => { setVintageYear(e.target.value); setPageNumber(1); }}
            label="Vintage"
          >
            <MenuItem value="">All</MenuItem>
            {vintages.map((v) => (
              <MenuItem key={v.vintageID} value={String(v.year)}>
                {v.year}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControlLabel
          control={
            <Switch
              checked={favorites}
              onChange={(e) => { setFavorites(e.target.checked); setPageNumber(1); }}
            />
          }
          label="Favorites"
        />

        <TextField
          label="Search"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") listQuery.refetch();
          }}
        />
        <Button
          variant="outlined"
          onClick={() => listQuery.refetch()}
        >
          Search
        </Button>

        {countQuery.data !== undefined && (
          <Typography variant="body1" sx={{ ml: "auto" }}>
            Total: {countQuery.data}
          </Typography>
        )}
      </Box>

      <DataTable
        title="Bottles"
        rows={rows}
        fields={bottleTableFields}
        isLoading={listQuery.isLoading}
        error={listQuery.error as Error | null}
        onCreate={handleCreate}
        onEdit={handleEdit}
        onDelete={isAdmin ? handleDelete : undefined}
        pagination={
          pagedData
            ? {
                pageNumber,
                pageSize,
                totalCount: pagedData.totalCount,
                totalPages: pagedData.totalPages,
                onPageChange: setPageNumber,
                onPageSizeChange: (size) => { setPageSize(size); setPageNumber(1); },
              }
            : undefined
        }
      />

      <CrudDialog
        open={dialogOpen}
        title={
          isCreate
            ? isAdmin
              ? "Create Bottle"
              : "Add Bottle to Cellar"
            : "Edit Bottle"
        }
        fields={isAdmin ? [...catalogFields, ...cellarFields] : cellarFields}
        values={dialogData}
        error={saveMutation.error as Error | null}
        extraFields={
          <>
            {cellarPicker}
            {catalogPickers}
          </>
        }
        onClose={() => setDialogOpen(false)}
        onSave={handleSave}
      />
      <QuantityDialog
        open={qtyBottle !== null}
        title={qtyBottle ? `Set ${qtyBottle.vineyard ?? "Bottle"} Quantity` : "Set Quantity"}
        label="Cellar quantity"
        confirmLabel="Save"
        min={0}
        initialValue={qtyBottle?.quantity}
        onClose={() => setQtyBottle(null)}
        onConfirm={(q) => {
          if (!qtyBottle) return;
          adjustQtyMutation.mutate({ id: qtyBottle.bottleID, quantity: q });
          setQtyBottle(null);
        }}
      />
    </Box>
  );
}
