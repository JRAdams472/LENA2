"use client";

import { use, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import TextField from "@mui/material/TextField";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import FormControlLabel from "@mui/material/FormControlLabel";
import IconButton from "@mui/material/IconButton";
import Chip from "@mui/material/Chip";
import Dialog from "@mui/material/Dialog";
import DialogTitle from "@mui/material/DialogTitle";
import DialogContent from "@mui/material/DialogContent";
import DialogActions from "@mui/material/DialogActions";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import DeleteIcon from "@mui/icons-material/Delete";
import DragIndicatorIcon from "@mui/icons-material/DragIndicator";
import MoreVertIcon from "@mui/icons-material/MoreVert";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import ArrowDownwardIcon from "@mui/icons-material/ArrowDownward";
import Alert from "@mui/material/Alert";
import CircularProgress from "@mui/material/CircularProgress";
import { api } from "@/lib/api";
import { fmtQty, sizeBadge, stripSize } from "@/lib/format";
import { GroceryListItem, GroceryRouteGroup, Store, StoreAisle } from "@/lib/types";

interface ManualForm {
  manualItemName: string;
  quantityNeeded: string;
  unitOfMeasure: string;
}

function itemName(it: GroceryListItem): string {
  return it.itemName ?? it.manualItemName ?? `Item ${it.itemID}`;
}

// reorderEntries builds the mutation payload for the post-drop display
// order: every item gets its rank; the moved item also carries the aisle
// it was dropped into (null aisle = list's unassigned bucket).
export function reorderEntries(
  groups: GroceryRouteGroup[],
  movedId: number,
  targetGroupIdx: number,
  targetItemIdx: number
): { groceryListItemID: number; aisleID?: number | null }[] {
  const flat: { it: GroceryListItem; groupIdx: number }[] = [];
  groups.forEach((g, gi) => g.items.forEach((ri) => flat.push({ it: ri.item, groupIdx: gi })));

  const from = flat.findIndex((f) => f.it.groceryListItemID === movedId);
  if (from === -1) return [];
  const moved = flat.splice(from, 1)[0];

  // Convert the drop target (group, item-index-within-group) into a flat
  // insert index, counting items still in the list after removal.
  let insertAt = flat.length;
  let seen = 0;
  for (let i = 0; i < flat.length; i++) {
    if (flat[i].groupIdx === targetGroupIdx) {
      if (seen === targetItemIdx) {
        insertAt = i;
        break;
      }
      seen++;
      insertAt = i + 1;
    }
  }
  flat.splice(Math.min(insertAt, flat.length), 0, { it: moved.it, groupIdx: targetGroupIdx });

  return flat.map((f) => ({
    groceryListItemID: f.it.groceryListItemID,
    // Only the moved row carries an aisleId — other items keep their
    // assignments; an unset aisleId means "no change", not "unassigned".
    aisleID: f.it.groceryListItemID === movedId ? groups[targetGroupIdx]?.aisle?.aisleID : undefined,
  }));
}

export function ItemRow({
  item,
  listId,
  suggested,
  draggable = false,
  aisles = [],
  onMoveToAisle,
  onDragStart,
  onDragOverRow,
}: {
  item: GroceryListItem;
  listId: number;
  suggested?: boolean;
  draggable?: boolean;
  aisles?: StoreAisle[];
  onMoveToAisle?: (item: GroceryListItem, aisleID: number | null) => void;
  onDragStart?: (itemID: number) => void;
  onDragOverRow?: (e: React.DragEvent, itemID: number) => void;
}) {
  const queryClient = useQueryClient();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);

  const toggleMutation = useMutation({
    mutationFn: () => api.toggleGroceryListItemChecked(item.groceryListItemID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["groceryList", listId] });
      queryClient.invalidateQueries({ queryKey: ["routeGroups", listId] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteGroceryListItem(item.groceryListItemID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["groceryList", listId] });
      queryClient.invalidateQueries({ queryKey: ["routeGroups", listId] });
    },
  });

  return (
    <Box
      data-testid={`grocery-row-${item.groceryListItemID}`}
      onDragOver={(e) => onDragOverRow?.(e, item.groceryListItemID)}
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1,
        p: 1,
        borderBottom: "1px solid",
        borderColor: "divider",
      }}
    >
      {draggable && (
        <IconButton
          size="small"
          aria-label="drag to reorder"
          draggable
          onDragStart={(e) => {
            e.dataTransfer.effectAllowed = "move";
            onDragStart?.(item.groceryListItemID);
          }}
          sx={{ cursor: "grab" }}
        >
          <DragIndicatorIcon fontSize="small" />
        </IconButton>
      )}
      <FormControlLabel
        control={
          <Checkbox
            checked={item.isChecked}
            onChange={() => toggleMutation.mutate()}
          />
        }
        label={
          <Box>
            <Typography
              variant="body1"
              sx={{
                textDecoration: item.isChecked ? "line-through" : "none",
              }}
            >
              {itemName(item)}
              {suggested && (
                <Chip
                  label="suggested aisle"
                  size="small"
                  variant="outlined"
                  sx={{ ml: 1 }}
                />
              )}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {fmtQty(Number(item.quantityNeeded))} {item.unitOfMeasure}
            </Typography>
          </Box>
        }
      />
      <Box sx={{ flexGrow: 1 }} />
      {aisles.length > 0 && (
        <>
          <IconButton
            size="small"
            aria-label="item actions"
            onClick={(e) => setMenuAnchor(e.currentTarget)}
          >
            <MoreVertIcon fontSize="small" />
          </IconButton>
          <Menu
            anchorEl={menuAnchor}
            open={Boolean(menuAnchor)}
            onClose={() => setMenuAnchor(null)}
          >
            {aisles.map((a) => (
              <MenuItem
                key={a.aisleID}
                onClick={() => {
                  setMenuAnchor(null);
                  onMoveToAisle?.(item, a.aisleID);
                }}
              >
                Move to {a.name}
              </MenuItem>
            ))}
            <MenuItem
              onClick={() => {
                setMenuAnchor(null);
                onMoveToAisle?.(item, null);
              }}
            >
              Move to unassigned
            </MenuItem>
          </Menu>
        </>
      )}
      <IconButton
        size="small"
        aria-label="delete item"
        onClick={() => deleteMutation.mutate()}
        disabled={deleteMutation.isPending}
      >
        <DeleteIcon fontSize="small" />
      </IconButton>
    </Box>
  );
}

// StoreLayoutDialog manages a store's aisle list: rename, delete, reorder
// via up/down arrows, and add new aisles.
function StoreLayoutDialog({
  store,
  onClose,
}: {
  store: Store;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [newAisle, setNewAisle] = useState("");
  const aisles = [...(store.aisles ?? [])].sort((a, b) => a.position - b.position);

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ["groceryStores"] });
    queryClient.invalidateQueries({ queryKey: ["routeGroups"] });
    queryClient.invalidateQueries({ queryKey: ["groceryList"] });
  };

  const addMutation = useMutation({
    mutationFn: () =>
      api.createStoreAisle(store.storeID, newAisle.trim(), aisles.length),
    onSuccess: () => {
      setNewAisle("");
      refresh();
    },
  });
  const renameMutation = useMutation({
    mutationFn: ({ aisleID, name }: { aisleID: number; name: string }) =>
      api.renameStoreAisle(aisleID, name),
    onSuccess: refresh,
  });
  const deleteMutation = useMutation({
    mutationFn: (aisleID: number) => api.deleteStoreAisle(aisleID),
    onSuccess: refresh,
  });
  const moveMutation = useMutation({
    mutationFn: (ids: number[]) => api.reorderStoreAisles(store.storeID, ids),
    onSuccess: refresh,
  });

  const move = (idx: number, dir: -1 | 1) => {
    const ids = aisles.map((a) => a.aisleID);
    const j = idx + dir;
    if (j < 0 || j >= ids.length) return;
    [ids[idx], ids[j]] = [ids[j], ids[idx]];
    moveMutation.mutate(ids);
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{store.name} — aisle layout</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Aisles are walked in this order. Drag items between aisle groups on
          the list to reassign them, or use an item&apos;s menu.
        </Typography>
        {aisles.map((a, idx) => (
          <Box
            key={a.aisleID}
            sx={{ display: "flex", alignItems: "center", gap: 0.5, py: 0.5 }}
          >
            <IconButton size="small" onClick={() => move(idx, -1)} disabled={idx === 0} aria-label="move aisle up">
              <ArrowUpwardIcon fontSize="small" />
            </IconButton>
            <IconButton size="small" onClick={() => move(idx, 1)} disabled={idx === aisles.length - 1} aria-label="move aisle down">
              <ArrowDownwardIcon fontSize="small" />
            </IconButton>
            <TextField
              size="small"
              defaultValue={a.name}
              sx={{ flexGrow: 1 }}
              onBlur={(e) => {
                const name = e.target.value.trim();
                if (name && name !== a.name) {
                  renameMutation.mutate({ aisleID: a.aisleID, name });
                }
              }}
            />
            <IconButton size="small" onClick={() => deleteMutation.mutate(a.aisleID)} aria-label="delete aisle">
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Box>
        ))}
        <Box sx={{ display: "flex", gap: 1, mt: 2 }}>
          <TextField
            size="small"
            label="New aisle"
            value={newAisle}
            sx={{ flexGrow: 1 }}
            onChange={(e) => setNewAisle(e.target.value)}
          />
          <Button
            variant="contained"
            onClick={() => addMutation.mutate()}
            disabled={newAisle.trim() === "" || addMutation.isPending}
          >
            Add
          </Button>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Done</Button>
      </DialogActions>
    </Dialog>
  );
}

export default function GroceryListDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const listId = Number(id);
  const queryClient = useQueryClient();
  const dragItemRef = useRef<number | null>(null);
  const [layoutStore, setLayoutStore] = useState<Store | null>(null);

  const listQuery = useQuery({
    queryKey: ["groceryList", listId],
    queryFn: () => api.getGroceryList(listId),
  });

  const storesQuery = useQuery({
    queryKey: ["groceryStores"],
    queryFn: () => api.getGroceryStores(),
  });

  const routeQuery = useQuery({
    queryKey: ["routeGroups", listId],
    queryFn: () => api.getGroceryRouteGroups(listId),
  });

  const restockQuery = useQuery({
    queryKey: ["suggestedRestock"],
    queryFn: () => api.getSuggestedRestockItems(10),
  });

  const setStoreMutation = useMutation({
    mutationFn: (storeId: number | null) => api.setGroceryListStore(listId, storeId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["groceryList", listId] });
      queryClient.invalidateQueries({ queryKey: ["routeGroups", listId] });
    },
  });

  const reorderMutation = useMutation({
    mutationFn: async ({
      movedId,
      targetGroupIdx,
      targetItemIdx,
    }: {
      movedId: number;
      targetGroupIdx: number;
      targetItemIdx: number;
    }) => {
      const entries = reorderEntries(groups, movedId, targetGroupIdx, targetItemIdx);
      if (entries.length === 0) return;
      // Dropping into the unassigned bucket must clear the item's aisle
      // explicitly — the reorder mutation treats a null aisleId as
      // "no change", so unassignment goes through assignItemToAisle.
      const targetIsUnassigned = groups[targetGroupIdx]?.aisle == null;
      const movedItem = groups
        .flatMap((g) => g.items)
        .find((ri) => ri.item.groceryListItemID === movedId);
      const wasAssigned =
        movedItem != null && !movedItem.suggested && groups.some(
          (g) => g.aisle != null && g.items.some((ri) => ri.item.groceryListItemID === movedId)
        );
      if (targetIsUnassigned && wasAssigned && store) {
        const it = movedItem.item;
        await api.assignItemToAisle(store.storeID, null, {
          itemID: it.itemID,
          ingredientID: it.ingredientID,
          manualItemName: it.manualItemName,
        });
      }
      await api.reorderGroceryListItems(listId, entries);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["routeGroups", listId] });
      queryClient.invalidateQueries({ queryKey: ["groceryList", listId] });
      queryClient.invalidateQueries({ queryKey: ["groceryStores"] });
    },
  });

  const addSuggestedMutation = useMutation({
    mutationFn: (itemId: number) =>
      api.addGroceryListItem(listId, {
        itemID: itemId,
        manualItemName: "",
        quantityNeeded: 1,
        unitOfMeasure: "",
        source: "pantry",
        isChecked: false,
      } as Omit<GroceryListItem, "groceryListItemID" | "groceryListID" | "groceryList">),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["groceryList", listId] });
      queryClient.invalidateQueries({ queryKey: ["routeGroups", listId] });
      queryClient.invalidateQueries({ queryKey: ["suggestedRestock"] });
    },
  });

  const [manual, setManual] = useState<ManualForm>({
    manualItemName: "",
    quantityNeeded: "",
    unitOfMeasure: "",
  });

  const addManualMutation = useMutation({
    mutationFn: () =>
      api.addGroceryListItem(listId, {
        itemID: null,
        manualItemName: manual.manualItemName,
        quantityNeeded: Number(manual.quantityNeeded),
        unitOfMeasure: manual.unitOfMeasure,
        source: "Manual",
        isChecked: false,
      } as Omit<GroceryListItem, "groceryListItemID" | "groceryListID" | "groceryList">),
    onSuccess: () => {
      setManual({ manualItemName: "", quantityNeeded: "", unitOfMeasure: "" });
      queryClient.invalidateQueries({ queryKey: ["groceryList", listId] });
      queryClient.invalidateQueries({ queryKey: ["routeGroups", listId] });
    },
  });

  const handleAddManual = () => {
    if (manual.manualItemName.trim() !== "" && manual.quantityNeeded !== "") {
      addManualMutation.mutate();
    }
  };

  const groups = routeQuery.data ?? [];
  const store = listQuery.data?.store ?? null;
  // The list's store field doesn't select aisles — they come from
  // groceryStores. store?.aisles maps missing to [] so fall back whenever
  // it is empty.
  const storeAisles = store?.aisles?.length
    ? store.aisles
    : storesQuery.data?.find((s) => s.storeID === store?.storeID)?.aisles ?? [];

  const submitOrder = (movedId: number, targetGroupIdx: number, targetItemIdx: number) => {
    reorderMutation.mutate({ movedId, targetGroupIdx, targetItemIdx });
  };

  const moveToAisle = (item: GroceryListItem, aisleID: number | null) => {
    // Append the item at the end of the target group.
    const targetIdx = groups.findIndex((g) => (g.aisle?.aisleID ?? null) === aisleID);
    const target = targetIdx === -1 ? groups.length - 1 : targetIdx;
    const itemCount = targetIdx === -1 ? 0 : groups[targetIdx].items.length;
    submitOrder(item.groceryListItemID, Math.max(target, 0), itemCount);
  };

  if (listQuery.isLoading) return <CircularProgress />;
  if (listQuery.error)
    return <Alert severity="error">{(listQuery.error as Error).message}</Alert>;
  if (!listQuery.data) return <Alert severity="warning">Grocery list not found</Alert>;

  const list = listQuery.data;

  return (
    <Box>
      <Paper sx={{ p: 3, mb: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
          <Box sx={{ flexGrow: 1 }}>
            <Typography variant="h4" gutterBottom>
              Grocery List
            </Typography>
            <Typography variant="body1" color="text.secondary">
              Generated {list.generatedDate?.split("T")[0]}
              {list.mealPlanID ? ` for Meal Plan ${list.mealPlanID}` : ""}
            </Typography>
          </Box>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="store-picker-label">Store</InputLabel>
            <Select
              labelId="store-picker-label"
              label="Store"
              value={store ? String(store.storeID) : ""}
              displayEmpty
              onChange={(e) =>
                setStoreMutation.mutate(e.target.value === "" ? null : Number(e.target.value))
              }
            >
              <MenuItem value="">No store</MenuItem>
              {(storesQuery.data ?? []).map((s) => (
                <MenuItem key={s.storeID} value={String(s.storeID)}>
                  {s.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          {store && (
            <Button
              variant="outlined"
              size="small"
              onClick={() => setLayoutStore(store)}
            >
              Edit aisles
            </Button>
          )}
        </Box>
        {setStoreMutation.error && (
          <Alert severity="error" sx={{ mt: 1 }}>
            {(setStoreMutation.error as Error).message}
          </Alert>
        )}
      </Paper>

      {routeQuery.isLoading ? (
        <CircularProgress />
      ) : (
        groups.filter((g) => g.items.length > 0).map((group, gi) => (
          <Paper key={group.aisle?.aisleID ?? "unassigned"} sx={{ p: 3, mb: 3 }}>
            <Typography variant="h5" gutterBottom>
              {group.aisle?.name ?? "Other items"}
            </Typography>
            {group.items.map((ri, ii) => (
              <Box
                key={ri.item.groceryListItemID}
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => {
                  e.preventDefault();
                  if (dragItemRef.current != null) {
                    submitOrder(dragItemRef.current, gi, ii);
                  }
                  dragItemRef.current = null;
                }}
              >
                <ItemRow
                  item={ri.item}
                  listId={listId}
                  suggested={ri.suggested}
                  draggable
                  aisles={storeAisles}
                  onMoveToAisle={moveToAisle}
                  onDragStart={(itemID) => {
                    dragItemRef.current = itemID;
                  }}
                  onDragOverRow={(e) => e.preventDefault()}
                />
              </Box>
            ))}
            {/* Empty tail drop zone — dropping after the last row appends
                the item at the end of this aisle group. */}
            <Box
              sx={{ minHeight: 8 }}
              onDragOver={(e) => e.preventDefault()}
              onDrop={(e) => {
                e.preventDefault();
                if (dragItemRef.current != null) {
                  submitOrder(dragItemRef.current, gi, group.items.length);
                }
                dragItemRef.current = null;
              }}
            />
          </Paper>
        ))
      )}
      {reorderMutation.error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {(reorderMutation.error as Error).message}
        </Alert>
      )}

      <Paper sx={{ p: 3, mb: 3 }}>
        <Typography variant="h5" gutterBottom>
          Jot something down
        </Typography>
        <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", mb: 2 }}>
          <TextField
            size="small"
            label="Item Name"
            value={manual.manualItemName}
            onChange={(e) =>
              setManual((m) => ({ ...m, manualItemName: e.target.value }))
            }
          />
          <TextField
            size="small"
            label="Qty"
            type="number"
            value={manual.quantityNeeded}
            onChange={(e) =>
              setManual((m) => ({ ...m, quantityNeeded: e.target.value }))
            }
          />
          <TextField
            size="small"
            label="Unit"
            value={manual.unitOfMeasure}
            onChange={(e) =>
              setManual((m) => ({ ...m, unitOfMeasure: e.target.value }))
            }
          />
          <Button
            variant="contained"
            onClick={handleAddManual}
            disabled={
              manual.manualItemName.trim() === "" || manual.quantityNeeded === ""
            }
          >
            Add
          </Button>
        </Box>
        {addManualMutation.error && (
          <Alert severity="error">
            {(addManualMutation.error as Error).message}
          </Alert>
        )}
      </Paper>

      {(restockQuery.data ?? []).length > 0 && (
        <Paper sx={{ p: 3, mb: 3 }}>
          <Typography variant="h5" gutterBottom>
            Time to restock
          </Typography>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            These pantry staples are at or below their minimum — ranked by
            how often your household uses them.
          </Typography>
          {(restockQuery.data ?? []).map((it) => {
            const size = sizeBadge(it.name, it.unit);
            return (
              <Box
                key={it.itemID}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  py: 0.5,
                }}
              >
                <Typography sx={{ flexGrow: 1 }}>
                  {stripSize(it.name, size, it.brand)}
                  {it.brand && (
                    <Typography component="span" color="text.secondary">
                      {" "}
                      — {it.brand}
                    </Typography>
                  )}
                </Typography>
                {size && (
                  <Chip
                    size="small"
                    variant="outlined"
                    label={size}
                    sx={{
                      color: "text.secondary",
                      borderColor: "divider",
                      "& .MuiChip-label": { px: 1.25 },
                    }}
                  />
                )}
                <Button
                  size="small"
                  variant="outlined"
                  disabled={addSuggestedMutation.isPending}
                  onClick={() => addSuggestedMutation.mutate(it.itemID)}
                >
                  Add
                </Button>
              </Box>
            );
          })}
        </Paper>
      )}

      {layoutStore && (
        <StoreLayoutDialog store={layoutStore} onClose={() => setLayoutStore(null)} />
      )}
    </Box>
  );
}
