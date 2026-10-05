"use client";

import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import EditIcon from "@mui/icons-material/Edit";
import DeleteIcon from "@mui/icons-material/Delete";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import { ReactNode, useMemo, useState } from "react";
import TableSortLabel from "@mui/material/TableSortLabel";
import { FieldDef } from "./CrudDialog";

interface DataTableProps<T extends object> {
  title: string;
  rows: T[];
  isLoading: boolean;
  error: Error | null;
  onCreate: () => void;
  onEdit: (row: T) => void;
  onDelete: (row: T) => void;
  extraActions?: (row: T) => ReactNode;
  page?: number;
  pageSize?: number;
  totalCount?: number;
  onPageChange?: (page: number) => void;
  onPageSizeChange?: (pageSize: number) => void;
  pagination?: {
    pageNumber: number;
    pageSize: number;
    totalCount: number;
    totalPages: number;
    onPageChange: (page: number) => void;
    onPageSizeChange: (size: number) => void;
  };
  fields?: FieldDef<T>[];
}

// Fallback cell rendering when a field has no custom render: objects
// stringify, nulls blank out.
function cellText(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

export default function DataTable<T extends object>({
  title,
  rows,
  isLoading,
  error,
  onCreate,
  onEdit,
  onDelete,
  extraActions,
  page,
  pageSize,
  totalCount,
  onPageChange,
  onPageSizeChange,
  pagination,
  fields,
}: DataTableProps<T>) {
  let columnDefs: FieldDef<T>[] = [];
  if (fields && fields.length > 0) {
    columnDefs = fields;
  } else if (rows.length > 0) {
    columnDefs = (Object.keys(rows[0]) as Extract<keyof T, string>[]).map(
      (key) => ({ key, label: key, sortable: true })
    );
  }

  const hiddenKeys = new Set(["createdBy", "createDate"]);
  const idRegex = /id$/i;
  columnDefs = columnDefs.filter((col) => !hiddenKeys.has(col.key) && !idRegex.test(col.key));

  const [sortField, setSortField] = useState<Extract<keyof T, string> | null>(null);
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");

  // Entities all expose a *-ID field; JSON is the last-resort key so rows
  // never key off their array index.
  const rowKey = (row: T): string | number => {
    const rec = row as Record<string, unknown>;
    for (const k of Object.keys(rec)) {
      const v = rec[k];
      if ((k === "id" || k.endsWith("ID") || k.endsWith("Id")) && (typeof v === "string" || typeof v === "number")) return v;
    }
    return JSON.stringify(rec);
  };

  const displayRows = useMemo(() => {
    if (!sortField) return rows;
    const sorted = [...rows];
    sorted.sort((a, b) => {
      const aVal = (a as Record<string, unknown>)[sortField];
      const bVal = (b as Record<string, unknown>)[sortField];
      let comparison = 0;

      if (aVal === null || aVal === undefined) comparison = 1;
      else if (bVal === null || bVal === undefined) comparison = -1;
      else if (typeof aVal === "number" && typeof bVal === "number") comparison = aVal - bVal;
      else if (typeof aVal === "boolean" && typeof bVal === "boolean") comparison = Number(aVal) - Number(bVal);
      else comparison = String(aVal).localeCompare(String(bVal));

      return sortDirection === "asc" ? comparison : -comparison;
    });
    return sorted;
  }, [rows, sortField, sortDirection]);

  const handleSort = (key: Extract<keyof T, string>) => {
    if (sortField === key) {
      setSortDirection((prev) => (prev === "asc" ? "desc" : "asc"));
    } else {
      setSortField(key);
      setSortDirection("asc");
    }
  };

  const paginationData =
    pagination ??
    (page !== undefined && pageSize !== undefined && totalCount !== undefined && onPageChange && onPageSizeChange
      ? {
          pageNumber: page,
          pageSize,
          totalCount,
          totalPages: Math.max(1, Math.ceil(totalCount / pageSize)),
          onPageChange,
          onPageSizeChange,
        }
      : null);

  return (
    <Box>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 2,
        }}
      >
        <Typography variant="h4" gutterBottom>
          {title}
        </Typography>
        <Button variant="contained" onClick={onCreate}>
          Create
        </Button>
      </Box>

      {isLoading && <CircularProgress />}
      {error && <Alert severity="error">{error.message}</Alert>}
      {!isLoading && !error && rows.length === 0 && (
        <Typography color="text.secondary">Nothing here yet</Typography>
      )}
      {!isLoading && !error && rows.length > 0 && (
        <TableContainer component={Paper}>
          <Table size="small">
            <TableHead>
              <TableRow>
                {columnDefs.map((col) => (
                  <TableCell key={col.key}>
                    {col.sortable !== false ? (
                      <TableSortLabel
                        active={sortField === col.key}
                        direction={sortDirection}
                        onClick={() => handleSort(col.key)}
                      >
                        {col.label}
                      </TableSortLabel>
                    ) : (
                      col.label
                    )}
                  </TableCell>
                ))}
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {displayRows.map((row) => (
                <TableRow key={rowKey(row)}>
                  {columnDefs.map((col) => {
                    const value = (row as Record<string, unknown>)[col.key];
                    return (
                      <TableCell key={col.key}>
                        {col.render ? col.render(row) : cellText(value)}
                      </TableCell>
                    );
                  })}
                  <TableCell>
                    <IconButton onClick={() => onEdit(row)} size="small" aria-label="Edit" data-testid="row-edit-button">
                      <EditIcon />
                    </IconButton>
                    <IconButton onClick={() => onDelete(row)} size="small" aria-label="Delete" data-testid="row-delete-button">
                      <DeleteIcon />
                    </IconButton>
                    {extraActions?.(row)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
      {paginationData && (
        <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            <FormControl size="small" sx={{ minWidth: 120 }}>
              <InputLabel id="rows-per-page-label">Rows per page</InputLabel>
              <Select
                labelId="rows-per-page-label"
                value={paginationData.pageSize}
                label="Rows per page"
                onChange={(e) => paginationData.onPageSizeChange(Number(e.target.value))}
              >
                <MenuItem value={10}>10</MenuItem>
                <MenuItem value={25}>25</MenuItem>
                <MenuItem value={50}>50</MenuItem>
                <MenuItem value={100}>100</MenuItem>
              </Select>
            </FormControl>
            <Typography variant="body2" color="text.secondary">
              Page {paginationData.pageNumber} of {Math.max(paginationData.totalPages, 1)} ({paginationData.totalCount} total)
            </Typography>
            <Button
              variant="outlined"
              size="small"
              onClick={() => paginationData.onPageChange(paginationData.pageNumber - 1)}
              disabled={paginationData.pageNumber <= 1}
            >
              &lt;
            </Button>
            <Button
              variant="outlined"
              size="small"
              onClick={() => paginationData.onPageChange(paginationData.pageNumber + 1)}
              disabled={paginationData.pageNumber >= Math.max(paginationData.totalPages, 1)}
            >
              &gt;
            </Button>
          </Box>
        </Box>
      )}
    </Box>
  );
}
