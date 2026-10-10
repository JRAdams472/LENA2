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
import FormControl from "@mui/material/FormControl";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
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
import { User } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";

const PAGE_SIZE = 25;

export default function UsersPage() {
  const { me, isAdmin, isLoading: meLoading, refetch: refetchMe } = useMe();
  const queryClient = useQueryClient();
  const [page, setPage] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<{ user: User; action: "ban" | "unban" } | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["admin-users", page],
    queryFn: () => api.getUsers(page + 1, PAGE_SIZE),
    enabled: isAdmin,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["admin-users"] });
    refetchMe();
  };

  const roleMutation = useMutation({
    mutationFn: ({ userId, role }: { userId: number; role: "member" | "admin" }) =>
      api.setUserRole(userId, role),
    onSuccess: () => {
      setError(null);
      invalidate();
    },
    onError: (e) =>
      setError(e instanceof ApiError ? e.message : "Failed to update role"),
  });

  const activeMutation = useMutation({
    mutationFn: ({ userId, isActive }: { userId: number; isActive: boolean }) =>
      api.setUserActive(userId, isActive),
    onSuccess: () => {
      setError(null);
      setConfirm(null);
      invalidate();
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : "Failed to update user");
      setConfirm(null);
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

  const fullName = (u: User) =>
    [u.firstName, u.lastName].filter(Boolean).join(" ") || u.displayName || "—";

  return (
    <Box>
      <Typography variant="h5" gutterBottom>
        Users
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
              <TableCell>Email</TableCell>
              <TableCell>Name</TableCell>
              <TableCell>Role</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Last login</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(data?.items ?? []).map((u) => {
              const isSelf = me?.userID === u.userID;
              const locked = isSelf || u.isProtected;
              const reason = isSelf
                ? "You cannot modify your own account"
                : "Protected admin";
              return (
                <TableRow key={u.userID}>
                  <TableCell>{u.email}</TableCell>
                  <TableCell>{fullName(u)}</TableCell>
                  <TableCell>
                    <FormControl size="small" disabled={locked || roleMutation.isPending}>
                      <Select
                        value={u.role}
                        onChange={(e) =>
                          roleMutation.mutate({
                            userId: u.userID,
                            role: e.target.value as "member" | "admin",
                          })
                        }
                      >
                        <MenuItem value="member">member</MenuItem>
                        <MenuItem value="admin">admin</MenuItem>
                      </Select>
                    </FormControl>
                  </TableCell>
                  <TableCell>
                    <Chip
                      size="small"
                      label={u.isActive ? "Active" : "Banned"}
                      color={u.isActive ? "success" : "error"}
                    />
                  </TableCell>
                  <TableCell>
                    {u.lastLoginAt
                      ? new Date(u.lastLoginAt).toLocaleString()
                      : "Never"}
                  </TableCell>
                  <TableCell align="right">
                    <Tooltip title={locked ? reason : ""}>
                      <span>
                        <Button
                          size="small"
                          color={u.isActive ? "error" : "primary"}
                          disabled={locked || activeMutation.isPending}
                          onClick={() =>
                            setConfirm({
                              user: u,
                              action: u.isActive ? "ban" : "unban",
                            })
                          }
                        >
                          {u.isActive ? "Ban" : "Unban"}
                        </Button>
                      </span>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination
        component="div"
        count={data?.totalCount ?? 0}
        page={page}
        onPageChange={(_, p) => setPage(p)}
        rowsPerPage={PAGE_SIZE}
        rowsPerPageOptions={[PAGE_SIZE]}
      />
      {isLoading && <CircularProgress size={24} sx={{ mt: 2 }} />}

      <Dialog open={confirm !== null} onClose={() => setConfirm(null)}>
        <DialogTitle>
          {confirm?.action === "ban" ? "Ban user?" : "Unban user?"}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {confirm?.action === "ban"
              ? `${confirm.user.email} will immediately lose API access. Their data is preserved and they can be unbanned later.`
              : `${confirm?.user.email} will regain API access.`}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirm(null)}>Cancel</Button>
          <Button
            color={confirm?.action === "ban" ? "error" : "primary"}
            variant="contained"
            onClick={() =>
              confirm &&
              activeMutation.mutate({
                userId: confirm.user.userID,
                isActive: confirm.action === "unban",
              })
            }
          >
            Confirm
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
