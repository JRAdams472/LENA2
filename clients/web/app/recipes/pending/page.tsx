"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import Link from "next/link";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TablePagination from "@mui/material/TablePagination";
import Typography from "@mui/material/Typography";
import { api } from "@/lib/api";
import { useMe } from "@/app/auth/useMe";

const PAGE_SIZE = 25;

function statusChip(status: string) {
  switch (status) {
    case "ready":
      return <Chip color="success" size="small" label="Ready" />;
    case "reviewing":
      return <Chip color="warning" size="small" label="Reviewing" />;
    case "profanity":
      return <Chip color="error" size="small" label="Profanity" />;
    case "failed":
      return <Chip color="error" size="small" label="Failed" />;
    case "pending":
    case "processing":
    case "ocred":
    case "drafted":
      return <Chip color="info" size="small" label="Processing" />;
    default:
      return <Chip size="small" label={status} />;
  }
}

export default function PendingRecipesPage() {
  const { isAdmin, isLoading: meLoading } = useMe();
  const [page, setPage] = useState(0);

  const { data, isLoading, error } = useQuery({
    queryKey: ["pending-recipe-imports", page],
    queryFn: () => api.getPendingRecipeImports(page + 1, PAGE_SIZE),
    enabled: isAdmin,
    refetchInterval: 5000,
  });

  if (meLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!isAdmin) {
    return <Alert severity="error">Forbidden: this page requires the admin role.</Alert>;
  }

  return (
    <Box>
      <Typography variant="h5" gutterBottom>
        Pending Recipe Reviews
      </Typography>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error instanceof Error ? error.message : "Failed to load pending imports"}
        </Alert>
      )}
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Filename</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Updated</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {data?.items.map((ri) => (
              <TableRow key={ri.recipeImportID}>
                <TableCell>{ri.sourceFilename}</TableCell>
                <TableCell>{statusChip(ri.status)}</TableCell>
                <TableCell>{ri.createDate ? new Date(ri.createDate).toLocaleString() : "—"}</TableCell>
                <TableCell>{ri.lastUpdatedDate ? new Date(ri.lastUpdatedDate).toLocaleString() : "—"}</TableCell>
                <TableCell align="right">
                  <Button size="small" component={Link} href={`/recipes/pending/${ri.recipeImportID}`}>
                    Review
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination
        component="div"
        count={data?.totalCount ?? 0}
        page={page}
        onPageChange={(_e, newPage) => setPage(newPage)}
        rowsPerPage={PAGE_SIZE}
        rowsPerPageOptions={[PAGE_SIZE]}
      />
      {isLoading && (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 2 }}>
          <CircularProgress size={24} />
        </Box>
      )}
    </Box>
  );
}
