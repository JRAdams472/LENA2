"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TablePagination from "@mui/material/TablePagination";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import { api, ApiError } from "@/lib/api";
import { fmtQty } from "@/lib/format";
import { Bottle, Item } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";
import ConfirmDialog from "@/app/components/ConfirmDialog";

const PAGE_SIZE = 25;

export default function PendingItemsPage() {
  const { isAdmin, isLoading: meLoading } = useMe();
  const queryClient = useQueryClient();
  const [page, setPage] = useState(0);
  const [bottlePage, setBottlePage] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<{ item: Item; action: "approve" | "reject" } | null>(null);
  const [bottleConfirm, setBottleConfirm] = useState<{ bottle: Bottle; action: "approve" | "reject" } | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["pending-items", page],
    queryFn: () => api.getPendingItems(page + 1, PAGE_SIZE),
    enabled: isAdmin,
  });

  const bottlesQuery = useQuery({
    queryKey: ["pending-bottles", bottlePage],
    queryFn: () => api.getPendingBottles(bottlePage + 1, PAGE_SIZE),
    enabled: isAdmin,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["pending-items"] });
    queryClient.invalidateQueries({ queryKey: ["pending-bottles"] });
  };

  const approveMutation = useMutation({
    mutationFn: (itemId: number) => api.approveItem(itemId),
    onSuccess: () => {
      setError(null);
      setConfirm(null);
      invalidate();
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : "Failed to approve item");
      setConfirm(null);
    },
  });

  const rejectMutation = useMutation({
    mutationFn: (itemId: number) => api.rejectItem(itemId),
    onSuccess: () => {
      setError(null);
      setConfirm(null);
      invalidate();
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : "Failed to reject item");
      setConfirm(null);
    },
  });

  const approveBottleMutation = useMutation({
    mutationFn: (bottleId: number) => api.approveBottle(bottleId),
    onSuccess: () => {
      setError(null);
      setBottleConfirm(null);
      invalidate();
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : "Failed to approve bottle");
      setBottleConfirm(null);
    },
  });

  const rejectBottleMutation = useMutation({
    mutationFn: (bottleId: number) => api.rejectBottle(bottleId),
    onSuccess: () => {
      setError(null);
      setBottleConfirm(null);
      invalidate();
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : "Failed to reject bottle");
      setBottleConfirm(null);
    },
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

  const upc = (item: Item) => item.upc12 ?? item.upc14 ?? "—";

  const nutrientSummary = (item: Item) => {
    if (!item.foodNutrients || item.foodNutrients.length === 0) return "—";
    const labels = item.foodNutrients.slice(0, 3).map((n) => {
      const name = n.nutrientType?.nutrientName ?? "Unknown";
      return `${name}: ${fmtQty(n.amountPerServing)}`;
    });
    const extra = item.foodNutrients.length > 3 ? ` (+${item.foodNutrients.length - 3} more)` : "";
    return labels.join(", ") + extra;
  };

  return (
    <Box>
      <Typography variant="h5" gutterBottom>
        Pending Items
      </Typography>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Brand</TableCell>
              <TableCell>Category</TableCell>
              <TableCell>UPC</TableCell>
              <TableCell>Nutrients</TableCell>
              <TableCell>Submitter</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(data?.items ?? []).map((item) => (
              <TableRow key={item.itemID}>
                <TableCell>{item.name}</TableCell>
                <TableCell>{item.brand ?? "—"}</TableCell>
                <TableCell>{item.category?.categoryName ?? "—"}</TableCell>
                <TableCell>{upc(item)}</TableCell>
                <TableCell>
                  <Tooltip title={nutrientSummary(item)}>
                    <Typography variant="body2" noWrap sx={{ maxWidth: 240 }}>
                      {nutrientSummary(item)}
                    </Typography>
                  </Tooltip>
                </TableCell>
                <TableCell>
                  {item.submittedByMe ? <Chip size="small" label="You" /> : "—"}
                </TableCell>
                <TableCell align="right">
                  <Button
                    size="small"
                    color="primary"
                    onClick={() => setConfirm({ item, action: "approve" })}
                    disabled={approveMutation.isPending || rejectMutation.isPending}
                  >
                    Approve
                  </Button>
                  <Button
                    size="small"
                    color="error"
                    onClick={() => setConfirm({ item, action: "reject" })}
                    disabled={approveMutation.isPending || rejectMutation.isPending}
                    sx={{ ml: 1 }}
                  >
                    Reject
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
      {!isLoading && (data?.items ?? []).length === 0 && (
        <Typography color="text.secondary" sx={{ mt: 2 }}>
          Nothing pending right now.
        </Typography>
      )}
      <TablePagination
        component="div"
        count={data?.totalCount ?? 0}
        page={page}
        onPageChange={(_, p) => setPage(p)}
        rowsPerPage={PAGE_SIZE}
        rowsPerPageOptions={[PAGE_SIZE]}
      />
      {isLoading && <CircularProgress size={24} sx={{ mt: 2 }} />}

      <Typography variant="h5" gutterBottom sx={{ mt: 4 }}>
        Pending Bottles
      </Typography>
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Vineyard</TableCell>
              <TableCell>Vintage</TableCell>
              <TableCell>Size</TableCell>
              <TableCell>ABV</TableCell>
              <TableCell>Submitter</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(bottlesQuery.data?.items ?? []).map((bottle) => (
              <TableRow key={bottle.bottleID}>
                <TableCell>{bottle.vineyard ?? "—"}</TableCell>
                <TableCell>{bottle.vintageYear}</TableCell>
                <TableCell>{bottle.bottleSize}</TableCell>
                <TableCell>{bottle.abv ?? "—"}</TableCell>
                <TableCell>
                  {bottle.submittedByMe ? <Chip size="small" label="You" /> : "—"}
                </TableCell>
                <TableCell align="right">
                  <Button
                    size="small"
                    color="primary"
                    onClick={() => setBottleConfirm({ bottle, action: "approve" })}
                    disabled={approveBottleMutation.isPending || rejectBottleMutation.isPending}
                  >
                    Approve
                  </Button>
                  <Button
                    size="small"
                    color="error"
                    onClick={() => setBottleConfirm({ bottle, action: "reject" })}
                    disabled={approveBottleMutation.isPending || rejectBottleMutation.isPending}
                    sx={{ ml: 1 }}
                  >
                    Reject
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
      {!bottlesQuery.isLoading && (bottlesQuery.data?.items ?? []).length === 0 && (
        <Typography color="text.secondary" sx={{ mt: 2 }}>
          No bottles pending right now.
        </Typography>
      )}
      <TablePagination
        component="div"
        count={bottlesQuery.data?.totalCount ?? 0}
        page={bottlePage}
        onPageChange={(_, p) => setBottlePage(p)}
        rowsPerPage={PAGE_SIZE}
        rowsPerPageOptions={[PAGE_SIZE]}
      />
      {bottlesQuery.isLoading && <CircularProgress size={24} sx={{ mt: 2 }} />}

      <Dialog open={confirm !== null} onClose={() => setConfirm(null)}>
        <DialogTitle>
          {confirm?.action === "approve" ? "Approve item?" : "Reject item?"}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {confirm?.action === "approve"
              ? `"${confirm.item.name}" will become visible to all users.`
              : `"${confirm?.item.name}" will be marked rejected and hidden from everyone.`}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirm(null)}>Cancel</Button>
          <Button
            color={confirm?.action === "reject" ? "error" : "primary"}
            variant="contained"
            onClick={() =>
              confirm &&
              (confirm.action === "approve"
                ? approveMutation.mutate(confirm.item.itemID)
                : rejectMutation.mutate(confirm.item.itemID))
            }
          >
            Confirm
          </Button>
        </DialogActions>
      </Dialog>

      <ConfirmDialog
        open={bottleConfirm !== null}
        title={bottleConfirm?.action === "approve" ? "Approve bottle?" : "Reject bottle?"}
        message={
          bottleConfirm?.action === "approve"
            ? `"${bottleConfirm?.bottle.vineyard ?? "Bottle"} ${bottleConfirm?.bottle.vintageYear ?? ""}" will become visible to all users.`
            : `"${bottleConfirm?.bottle.vineyard ?? "Bottle"} ${bottleConfirm?.bottle.vintageYear ?? ""}" will be marked rejected and hidden from everyone.`
        }
        destructive={bottleConfirm?.action === "reject"}
        busy={approveBottleMutation.isPending || rejectBottleMutation.isPending}
        onClose={() => setBottleConfirm(null)}
        onConfirm={() =>
          bottleConfirm &&
          (bottleConfirm.action === "approve"
            ? approveBottleMutation.mutate(bottleConfirm.bottle.bottleID)
            : rejectBottleMutation.mutate(bottleConfirm.bottle.bottleID))
        }
      />
    </Box>
  );
}
