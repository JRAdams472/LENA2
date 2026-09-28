"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
import IconButton from "@mui/material/IconButton";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Paper from "@mui/material/Paper";
import Select from "@mui/material/Select";
import Switch from "@mui/material/Switch";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import { api, ApiError } from "@/lib/api";
import { RecipeCategoryGroup } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";

export default function RecipeCategoriesPage() {
  const { isAdmin } = useMe();
  const queryClient = useQueryClient();

  const [selectedGroupId, setSelectedGroupId] = useState<number | "">("");

  const [groupDialogOpen, setGroupDialogOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<RecipeCategoryGroup | null>(null);
  const [groupName, setGroupName] = useState("");
  const [groupExclusive, setGroupExclusive] = useState(true);
  const [groupOrder, setGroupOrder] = useState("0");

  const [categoryDialogOpen, setCategoryDialogOpen] = useState(false);
  const [editingCategoryId, setEditingCategoryId] = useState<number | null>(null);
  const [categoryName, setCategoryName] = useState("");

  const [error, setError] = useState<string | null>(null);

  const groupsQuery = useQuery({
    queryKey: ["recipe-category-groups"],
    queryFn: api.getRecipeCategoryGroups,
  });

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["recipe-category-groups"] });

  const onError = (e: unknown) =>
    setError(e instanceof ApiError ? e.message : (e as Error).message);

  const saveGroupMutation = useMutation({
    mutationFn: () =>
      editingGroup
        ? api.updateRecipeCategoryGroup(editingGroup.categoryGroupID, {
            name: groupName.trim(),
            exclusive: groupExclusive,
            displayOrder: Number(groupOrder) || 0,
          })
        : api.createRecipeCategoryGroup({
            name: groupName.trim(),
            exclusive: groupExclusive,
            displayOrder: Number(groupOrder) || 0,
          }),
    onSuccess: () => {
      setGroupDialogOpen(false);
      setError(null);
      return invalidate();
    },
    onError,
  });

  const deleteGroupMutation = useMutation({
    mutationFn: (id: number) => api.deleteRecipeCategoryGroup(id),
    onSuccess: () => {
      setError(null);
      return invalidate();
    },
    onError,
  });

  const saveCategoryMutation = useMutation({
    mutationFn: () =>
      editingCategoryId
        ? api.updateRecipeCategory(editingCategoryId, categoryName.trim())
        : api.createRecipeCategory(Number(selectedGroupId), categoryName.trim()),
    onSuccess: () => {
      setCategoryDialogOpen(false);
      setError(null);
      return invalidate();
    },
    onError,
  });

  const deleteCategoryMutation = useMutation({
    mutationFn: (id: number) => api.deleteRecipeCategory(id),
    onSuccess: () => {
      setError(null);
      return invalidate();
    },
    onError,
  });

  const openCreateGroup = () => {
    setEditingGroup(null);
    setGroupName("");
    setGroupExclusive(true);
    setGroupOrder(String((groupsQuery.data ?? []).length + 1));
    setGroupDialogOpen(true);
  };

  const openEditGroup = (g: RecipeCategoryGroup) => {
    setEditingGroup(g);
    setGroupName(g.groupName);
    setGroupExclusive(g.exclusive);
    setGroupOrder(String(g.displayOrder));
    setGroupDialogOpen(true);
  };

  const openCreateCategory = () => {
    setEditingCategoryId(null);
    setCategoryName("");
    setCategoryDialogOpen(true);
  };

  const openEditCategory = (id: number, name: string) => {
    setEditingCategoryId(id);
    setCategoryName(name);
    setCategoryDialogOpen(true);
  };

  const selectedGroup = (groupsQuery.data ?? []).find(
    (g) => g.categoryGroupID === Number(selectedGroupId)
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h4">Recipe Categories</Typography>
      {!isAdmin && (
        <Alert severity="info">
          Only admins can edit the recipe taxonomy.
        </Alert>
      )}
      {error && (
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <Paper sx={{ p: 3 }}>
        <Box sx={{ display: "flex", justifyContent: "space-between", mb: 2 }}>
          <Typography variant="h5">Groups</Typography>
          {isAdmin && (
            <Button variant="contained" size="small" onClick={openCreateGroup}>
              New Group
            </Button>
          )}
        </Box>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Exclusive</TableCell>
              <TableCell>Order</TableCell>
              <TableCell>Categories</TableCell>
              {isAdmin && <TableCell align="right">Actions</TableCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {(groupsQuery.data ?? []).map((g) => (
              <TableRow key={g.categoryGroupID}>
                <TableCell>{g.groupName}</TableCell>
                <TableCell>{g.exclusive ? "Yes" : "No"}</TableCell>
                <TableCell>{g.displayOrder}</TableCell>
                <TableCell>{g.categories.length}</TableCell>
                {isAdmin && (
                  <TableCell align="right">
                    <IconButton
                      size="small"
                      aria-label={`edit ${g.groupName}`}
                      onClick={() => openEditGroup(g)}
                    >
                      <EditIcon fontSize="small" />
                    </IconButton>
                    <IconButton
                      size="small"
                      aria-label={`delete ${g.groupName}`}
                      onClick={() => deleteGroupMutation.mutate(g.categoryGroupID)}
                    >
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Paper>

      <Paper sx={{ p: 3 }}>
        <Box sx={{ display: "flex", gap: 2, alignItems: "center", mb: 2 }}>
          <Typography variant="h5">Categories</Typography>
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <InputLabel id="group-select-label">Group</InputLabel>
            <Select
              labelId="group-select-label"
              label="Group"
              value={selectedGroupId === "" ? "" : String(selectedGroupId)}
              onChange={(e) =>
                setSelectedGroupId(
                  e.target.value === "" ? "" : Number(e.target.value)
                )
              }
            >
              <MenuItem value="">
                <em>Choose a group</em>
              </MenuItem>
              {(groupsQuery.data ?? []).map((g) => (
                <MenuItem key={g.categoryGroupID} value={String(g.categoryGroupID)}>
                  {g.groupName}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          {isAdmin && selectedGroup && (
            <Button variant="contained" size="small" onClick={openCreateCategory}>
              New Category
            </Button>
          )}
        </Box>
        {selectedGroup && (
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                {isAdmin && <TableCell align="right">Actions</TableCell>}
              </TableRow>
            </TableHead>
            <TableBody>
              {selectedGroup.categories.map((c) => (
                <TableRow key={c.categoryID}>
                  <TableCell>{c.categoryName}</TableCell>
                  {isAdmin && (
                    <TableCell align="right">
                      <IconButton
                        size="small"
                        aria-label={`edit ${c.categoryName}`}
                        onClick={() => openEditCategory(c.categoryID, c.categoryName)}
                      >
                        <EditIcon fontSize="small" />
                      </IconButton>
                      <IconButton
                        size="small"
                        aria-label={`delete ${c.categoryName}`}
                        onClick={() => deleteCategoryMutation.mutate(c.categoryID)}
                      >
                        <DeleteIcon fontSize="small" />
                      </IconButton>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </Paper>

      <Dialog open={groupDialogOpen} onClose={() => setGroupDialogOpen(false)}>
        <DialogTitle>{editingGroup ? "Edit Group" : "New Group"}</DialogTitle>
        <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 2 }}>
          <TextField
            label="Name"
            size="small"
            value={groupName}
            onChange={(e) => setGroupName(e.target.value)}
          />
          <TextField
            label="Display order"
            size="small"
            type="number"
            value={groupOrder}
            onChange={(e) => setGroupOrder(e.target.value)}
          />
          <FormControlLabel
            control={
              <Switch
                checked={groupExclusive}
                onChange={(e) => setGroupExclusive(e.target.checked)}
              />
            }
            label="Exclusive (a recipe picks at most one)"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setGroupDialogOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            disabled={!groupName.trim() || saveGroupMutation.isPending}
            onClick={() => saveGroupMutation.mutate()}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={categoryDialogOpen} onClose={() => setCategoryDialogOpen(false)}>
        <DialogTitle>{editingCategoryId ? "Rename Category" : "New Category"}</DialogTitle>
        <DialogContent sx={{ pt: 2 }}>
          <TextField
            label="Name"
            size="small"
            fullWidth
            value={categoryName}
            onChange={(e) => setCategoryName(e.target.value)}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCategoryDialogOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            disabled={!categoryName.trim() || saveCategoryMutation.isPending}
            onClick={() => saveCategoryMutation.mutate()}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
