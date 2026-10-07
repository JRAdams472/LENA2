"use client";

import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Skeleton from "@mui/material/Skeleton";
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
import useMediaQuery from "@mui/material/useMediaQuery";
import { useTheme } from "@mui/material/styles";
import { ReactNode, useMemo, useState } from "react";
import TableSortLabel from "@mui/material/TableSortLabel";
import Fade from "@mui/material/Fade";
import InboxOutlinedIcon from "@mui/icons-material/InboxOutlined";
import { FieldDef } from "./CrudDialog";
import EmptyState from "./EmptyState";

interface DataTableProps<T extends object> {
  title: string;
  rows: T[];
  isLoading: boolean;
  error: Error | null;
  onCreate: () => void;
  onEdit?: (row: T) => void;
  onDelete?: (row: T) => void;
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
  return String(value as string | number | boolean);
}

// Type-aware display value: booleans and ISO date/datetime strings get
// human text instead of raw JSON wire forms.
function displayText(value: unknown, type?: FieldDef<object>["type"]): string {
  if (value === null || value === undefined) return "";
  if (type === "boolean" || typeof value === "boolean") {
    return value ? "Yes" : "No";
  }
  // Date components ("2024-01-01" or ISO timestamps) render as a local
  // date — Date.parse treats date-only strings as UTC and would shift the
  // rendered day back in western timezones.
  if (typeof value === "string") {
    const dm = /^(\d{4})-(\d{2})-(\d{2})(?:T\d{2}:\d{2})?/.exec(value);
    if (dm && (type === "date" || value.includes("T"))) {
      return new Date(
        Number(dm[1]),
        Number(dm[2]) - 1,
        Number(dm[3])
      ).toLocaleDateString();
    }
  }
  return cellText(value);
}

function TableSkeleton({ columns, actions }: { columns: number; actions: boolean }) {
  const cols = columns + (actions ? 1 : 0);
  return (
    <TableContainer component={Paper} role="status" aria-label="Loading rows">
      <Table size="small">
        <TableBody>
          {Array.from({ length: 5 }).map((_, i) => (
            // eslint-disable-next-line @eslint-react/no-array-index-key -- placeholder rows have no identity
            <TableRow key={i}>
              {Array.from({ length: cols }).map((__, j) => (
                // eslint-disable-next-line @eslint-react/no-array-index-key -- placeholder cells have no identity
                <TableCell key={j}>
                  <Skeleton variant="text" width={j === 0 ? "60%" : "80%"} />
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
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
  // Last-resort header text when a page doesn't pass fields: "weekStartDate"
  // → "Week Start Date". Pages should pass fields for real labels.
  const humanize = (key: string) =>
    key
      .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
      .replace(/^./, (c) => c.toUpperCase());

  let columnDefs: FieldDef<T>[] = [];
  if (fields && fields.length > 0) {
    // Explicit fields are already curated — keys like "foodId" often back a
    // render that shows a related entity's name, so never hide them.
    columnDefs = fields;
  } else if (rows.length > 0) {
    // Inferred columns: hide raw ID/audit fields, humanize the rest.
    const hiddenKeys = new Set([
      "createdBy",
      "createDate",
      "lastUpdatedBy",
      "lastUpdatedDate",
    ]);
    const idRegex = /id$/i;
    columnDefs = (Object.keys(rows[0]) as Extract<keyof T, string>[])
      .filter((key) => !hiddenKeys.has(key) && !idRegex.test(key))
      .map((key) => ({ key, label: humanize(key), sortable: true }));
  }

  const theme = useTheme();
  const isNarrow = useMediaQuery(theme.breakpoints.down("sm"));
  const hasActions = !!(onEdit || onDelete || extraActions);

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
      else comparison = cellText(aVal).localeCompare(cellText(bVal));

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

      {isLoading && (
        <TableSkeleton
          columns={Math.max(columnDefs.length, 3)}
          actions={hasActions}
        />
      )}
      {error && <Alert severity="error">{error.message}</Alert>}
      {!isLoading && !error && rows.length === 0 && (
        (totalCount ?? pagination?.totalCount) ? (
          // Server-paged tables can legitimately return an empty page —
          // e.g. food flavors page over items, and this page's items have
          // none. Don't claim the dataset is empty.
          <Paper variant="outlined" sx={{ p: 4, textAlign: "center" }}>
            <Typography color="text.secondary">
              No results on this page — try another page.
            </Typography>
          </Paper>
        ) : (
          // No CTA here — the header's Create button is already the affordance.
          <EmptyState
            icon={InboxOutlinedIcon}
            title="Nothing here yet"
            description={`Create the first ${title.toLowerCase().replace(/s$/, "")} to get started.`}
          />
        )
      )}
      {!isLoading && !error && rows.length > 0 && isNarrow && (
        // Skeleton unmounts as content fades in — the spec'd crossfade.
        <Fade in>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {displayRows.map((row) => (
            <Paper key={rowKey(row)} variant="outlined" sx={{ p: 2 }}>
              {columnDefs.map((col) => {
                const value = (row as Record<string, unknown>)[col.key];
                return (
                  <Box
                    key={col.key}
                    sx={{
                      display: "flex",
                      justifyContent: "space-between",
                      gap: 2,
                      py: 0.5,
                    }}
                  >
                    <Typography variant="caption" color="text.secondary">
                      {col.label}
                    </Typography>
                    <Typography
                      variant="body2"
                      component="div"
                      sx={{ textAlign: "right" }}
                    >
                      {col.render ? col.render(row) : displayText(value, col.type)}
                    </Typography>
                  </Box>
                );
              })}
              {hasActions && (
                <Box
                  sx={{ display: "flex", justifyContent: "flex-end", gap: 1, mt: 1 }}
                >
                  {onEdit && (
                    <IconButton onClick={() => onEdit(row)} size="small" aria-label="Edit" data-testid="row-edit-button">
                      <EditIcon />
                    </IconButton>
                  )}
                  {onDelete && (
                    <IconButton onClick={() => onDelete(row)} size="small" aria-label="Delete" data-testid="row-delete-button">
                      <DeleteIcon />
                    </IconButton>
                  )}
                  {extraActions?.(row)}
                </Box>
              )}
            </Paper>
          ))}
        </Box>
        </Fade>
      )}
      {!isLoading && !error && rows.length > 0 && !isNarrow && (
        <Fade in>
        <TableContainer component={Paper}>
          <Table size="small">
            <TableHead>
              <TableRow>
                {columnDefs.map((col) => (
                  <TableCell
                    key={col.key}
                    sx={col.minWidth ? { minWidth: col.minWidth } : undefined}
                  >
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
                {hasActions && <TableCell>Actions</TableCell>}
              </TableRow>
            </TableHead>
            <TableBody>
              {displayRows.map((row) => (
                <TableRow key={rowKey(row)}>
                  {columnDefs.map((col) => {
                    const value = (row as Record<string, unknown>)[col.key];
                    return (
                      <TableCell
                        key={col.key}
                        sx={col.minWidth ? { minWidth: col.minWidth } : undefined}
                      >
                        <Box
                          // Long names clamp at two lines instead of ballooning
                          // the row height (W-24).
                          sx={{
                            display: "-webkit-box",
                            WebkitBoxOrient: "vertical",
                            WebkitLineClamp: 2,
                            overflow: "hidden",
                          }}
                        >
                          {col.render ? col.render(row) : displayText(value, col.type)}
                        </Box>
                      </TableCell>
                    );
                  })}
                  {hasActions && (
                    <TableCell>
                      <Box
                        sx={{
                          display: "flex",
                          flexWrap: "nowrap",
                          alignItems: "center",
                          gap: 0.5,
                        }}
                      >
                        {onEdit && (
                          <IconButton onClick={() => onEdit(row)} size="small" aria-label="Edit" data-testid="row-edit-button">
                            <EditIcon />
                          </IconButton>
                        )}
                        {onDelete && (
                          <IconButton onClick={() => onDelete(row)} size="small" aria-label="Delete" data-testid="row-delete-button">
                            <DeleteIcon />
                          </IconButton>
                        )}
                        {extraActions?.(row)}
                      </Box>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
        </Fade>
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
