"use client";

import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import Typography from "@mui/material/Typography";

interface PaginationFooterProps {
  page: number;
  pageSize: number;
  totalCount: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}

export default function PaginationFooter({
  page,
  pageSize,
  totalCount,
  onPageChange,
  onPageSizeChange,
}: PaginationFooterProps) {
  const totalPages = Math.max(1, Math.ceil(totalCount / pageSize) || 0);

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        mt: 2,
        flexWrap: "wrap",
        gap: 2,
      }}
    >
      <Typography variant="body2" color="text.secondary">
        Page {page} of {totalPages} ({totalCount} total)
      </Typography>

      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Button
          variant="outlined"
          size="small"
          onClick={() => onPageChange(page - 1)}
          disabled={page <= 1}
        >
          &lt;
        </Button>
        <Button
          variant="outlined"
          size="small"
          onClick={() => onPageChange(page + 1)}
          disabled={page >= totalPages || totalCount === 0}
        >
          &gt;
        </Button>
      </Box>

      <FormControl size="small" sx={{ minWidth: 120 }}>
        <InputLabel id="page-size-label">Per page</InputLabel>
        <Select
          labelId="page-size-label"
          value={pageSize}
          label="Per page"
          onChange={(e) => onPageSizeChange(Number(e.target.value))}
        >
          <MenuItem value={10}>10</MenuItem>
          <MenuItem value={25}>25</MenuItem>
          <MenuItem value={50}>50</MenuItem>
          <MenuItem value={100}>100</MenuItem>
        </Select>
      </FormControl>
    </Box>
  );
}
