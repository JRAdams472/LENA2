"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import useMediaQuery from "@mui/material/useMediaQuery";
import { useTheme } from "@mui/material/styles";
import Link from "next/link";
import { api } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import { FieldDef } from "@/app/components/CrudDialog";
import { GroceryList } from "@/lib/types";

// Sentinel for "Manual list (no meal plan)" in the create dialog's plan
// picker — a real selection carries the plan's id string.
const MANUAL = "";

type ListRow = {
  groceryListID: number;
  name: string;
  generatedDate: string;
  store: string;
  itemCount: string;
};

const listTableFields: FieldDef<ListRow>[] = [
  { key: "name", label: "List" },
  { key: "generatedDate", label: "Generated", type: "date" },
  { key: "store", label: "Store" },
  { key: "itemCount", label: "Items" },
];

export default function GroceryListsPage() {
  const queryClient = useQueryClient();
  const router = useRouter();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState<string>(MANUAL);

  const theme = useTheme();
  const fullScreen = useMediaQuery(theme.breakpoints.down("sm"));

  const listQuery = useQuery({
    queryKey: ["groceryLists"],
    queryFn: () => api.getGroceryLists(),
  });

  const plansQuery = useQuery({
    queryKey: ["mealPlans"],
    queryFn: () => api.getMealPlansPaged(1, 100),
  });

  const toRow = (list: GroceryList): ListRow => ({
    groceryListID: list.groceryListID,
    name: list.mealPlanName ?? "Manual list",
    generatedDate: list.generatedDate?.split("T")[0] ?? "",
    store: list.store?.name ?? "—",
    itemCount:
      list.groceryListItems === undefined
        ? "—"
        : `${list.groceryListItems.length} items`,
  });

  const generateMutation = useMutation({
    mutationFn: () =>
      api.generateGroceryList(
        selectedPlan === MANUAL ? null : Number(selectedPlan)
      ),
    onSuccess: (list) => {
      // Close + navigate only on success — a failed save keeps the dialog
      // open so its error surfaces instead of failing silently (LEN-139).
      setDialogOpen(false);
      queryClient.invalidateQueries({ queryKey: ["groceryLists"] });
      router.push(`/member/grocery-lists/${list.groceryListID}`);
    },
  });

  const handleCreate = () => {
    setSelectedPlan(MANUAL);
    generateMutation.reset();
    setDialogOpen(true);
  };

  const extraActions = (row: ListRow) => (
    <Button
      size="small"
      component={Link}
      href={`/member/grocery-lists/${row.groceryListID as number}`}
    >
      View
    </Button>
  );

  return (
    <Box>
      <DataTable
        title="Grocery Lists"
        rows={(listQuery.data ?? []).map(toRow)}
        fields={listTableFields}
        isLoading={listQuery.isLoading}
        error={listQuery.error as Error | null}
        onCreate={handleCreate}
        extraActions={extraActions}
      />
      <Dialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        fullWidth
        fullScreen={fullScreen}
        maxWidth="sm"
      >
        <DialogTitle>New Grocery List</DialogTitle>
        <DialogContent>
          {generateMutation.error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {(generateMutation.error as Error).message}
            </Alert>
          )}
          <FormControl fullWidth margin="dense">
            <InputLabel id="grocery-plan-label">Meal plan</InputLabel>
            <Select
              labelId="grocery-plan-label"
              label="Meal plan"
              value={selectedPlan}
              displayEmpty
              onChange={(e) => setSelectedPlan(e.target.value)}
            >
              <MenuItem value={MANUAL}>Manual list (no meal plan)</MenuItem>
              {(plansQuery.data?.items ?? []).map((p) => (
                <MenuItem key={p.mealPlanID} value={String(p.mealPlanID)}>
                  {p.planName}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>Cancel</Button>
          <Button
            onClick={() => generateMutation.mutate()}
            variant="contained"
            disabled={generateMutation.isPending}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
