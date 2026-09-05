"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
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
import CrudDialog from "@/app/components/CrudDialog";
import { Bottle, PagedResult } from "@/lib/types";

const bottleFields = [
  { key: "bottleNumber", label: "Bottle Number", type: "number" as const },
  { key: "typeID", label: "Type ID", type: "number" as const },
  { key: "countryID", label: "Country ID", type: "number" as const },
  { key: "regionID", label: "Region ID", type: "number" as const },
  { key: "vintageYear", label: "Vintage Year", type: "number" as const },
  { key: "vineyard", label: "Vineyard" },
  { key: "abv", label: "ABV", type: "number" as const },
  { key: "acidity", label: "Acidity", type: "number" as const },
  { key: "tanninLevel", label: "Tannin Level", type: "number" as const },
  { key: "body", label: "Body", type: "number" as const },
  { key: "sweetness", label: "Sweetness", type: "number" as const },
  {
    key: "oakIntegration",
    label: "Oak Integration",
    type: "boolean" as const,
  },
  { key: "bottleSize", label: "Bottle Size" },
  { key: "quantity", label: "Quantity", type: "number" as const },
  { key: "purchaseDate", label: "Purchase Date" },
  { key: "purchasePrice", label: "Purchase Price", type: "number" as const },
  { key: "storageTemp", label: "Storage Temp", type: "number" as const },
  { key: "location", label: "Location" },
  { key: "notes", label: "Notes" },
  { key: "isFavorite", label: "Favorite", type: "boolean" as const },
];

export default function BottlesPage() {
  const queryClient = useQueryClient();
  const [countryId, setCountryId] = useState<string>("");
  const [regionId, setRegionId] = useState<string>("");
  const [typeId, setTypeId] = useState<string>("");
  const [vintageYear, setVintageYear] = useState<string>("");
  const [favorites, setFavorites] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(25);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogData, setDialogData] = useState<Record<string, unknown>>({});
  const [isCreate, setIsCreate] = useState(false);

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

  const listQuery = useQuery<Bottle[] | PagedResult<Bottle>>({
    queryKey: [
      "bottles",
      countryId,
      regionId,
      typeId,
      vintageYear,
      favorites,
      searchTerm,
      pageNumber,
      pageSize,
    ],
    queryFn: () => {
      if (favorites) return api.getFavoriteBottles();
      if (searchTerm.trim()) return api.searchBottles(searchTerm.trim());
      if (countryId) return api.getBottlesByCountryId(Number(countryId));
      if (regionId) return api.getBottlesByRegionId(Number(regionId));
      if (typeId) return api.getBottlesByTypeId(Number(typeId));
      if (vintageYear) return api.getBottlesByVintageYear(Number(vintageYear));
      return api.getBottlesPaged(pageNumber, pageSize);
    },
    placeholderData: (prev) => prev,
  });

  const countQuery = useQuery({
    queryKey: ["bottle-count"],
    queryFn: api.getBottleCount,
  });

  const createMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.createBottle(asEntity(row)),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ["bottles"] }),
  });

  const updateMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.updateBottle(row.bottleID as number, asEntity(row)),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ["bottles"] }),
  });

  const deleteMutation = useMutation({
    mutationFn: (row: Bottle) => api.deleteBottle(row.bottleID),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ["bottles"] }),
  });

  const handleCreate = () => {
    setIsCreate(true);
    setDialogData({});
    setDialogOpen(true);
  };

  const handleEdit = (row: Bottle) => {
    setIsCreate(false);
    setDialogData({ ...row });
    setDialogOpen(true);
  };

  const handleDelete = (row: Bottle) => {
    if (window.confirm("Delete this bottle?")) {
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

  const countries = countriesQuery.data ?? [];
  const regions = regionsQuery.data ?? [];
  const types = typesQuery.data ?? [];
  const vintages = vintagesQuery.data ?? [];

  const listData = listQuery.data;
  const pagedData = listData && !Array.isArray(listData) ? (listData as PagedResult<Bottle>) : undefined;
  const rows = pagedData?.items ?? (listData as Bottle[] | undefined) ?? [];
  const isDefaultList =
    !favorites &&
    !searchTerm.trim() &&
    !countryId &&
    !regionId &&
    !typeId &&
    !vintageYear;

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
          <InputLabel>Country</InputLabel>
          <Select
            value={countryId}
            onChange={(e) => setCountryId(e.target.value)}
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
          <InputLabel>Region</InputLabel>
          <Select
            value={regionId}
            onChange={(e) => setRegionId(e.target.value)}
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
          <InputLabel>Type</InputLabel>
          <Select
            value={typeId}
            onChange={(e) => setTypeId(e.target.value)}
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
          <InputLabel>Vintage</InputLabel>
          <Select
            value={vintageYear}
            onChange={(e) => setVintageYear(e.target.value)}
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
              onChange={(e) => setFavorites(e.target.checked)}
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
        isLoading={listQuery.isLoading}
        error={listQuery.error as Error | null}
        onCreate={handleCreate}
        onEdit={handleEdit}
        onDelete={handleDelete}
        pagination={
          isDefaultList && pagedData
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
        title={isCreate ? "Create Bottle" : "Edit Bottle"}
        fields={bottleFields}
        values={dialogData}
        onClose={() => setDialogOpen(false)}
        onSave={handleSave}
      />
    </Box>
  );
}
