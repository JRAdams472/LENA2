"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Link from "next/link";
import { api } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";
import { GroceryList } from "@/lib/types";

const generateFields: FieldDef<GroceryList>[] = [
  { key: "mealPlanID", label: "Meal Plan ID (optional)", type: "number" },
];

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
  const [dialogData, setDialogData] = useState<Record<string, unknown>>({});

  const listQuery = useQuery({
    queryKey: ["groceryLists"],
    queryFn: () => api.getGroceryLists(),
  });

  const toRow = (list: GroceryList): ListRow => ({
    groceryListID: list.groceryListID,
    name: `List #${list.groceryListID}`,
    generatedDate: list.generatedDate?.split("T")[0] ?? "",
    store: list.store?.name ?? "—",
    itemCount:
      list.groceryListItems === undefined
        ? "—"
        : `${list.groceryListItems.length} items`,
  });

  const generateMutation = useMutation({
    mutationFn: (values: Record<string, unknown>) =>
      api.generateGroceryList(
        values.mealPlanID === "" ? undefined : (values.mealPlanID as number)
      ),
    onSuccess: (list) => {
      queryClient.invalidateQueries({ queryKey: ["groceryLists"] });
      router.push(`/grocery-lists/${list.groceryListID}`);
    },
  });

  const handleCreate = () => {
    setDialogData({ mealPlanID: "" });
    setDialogOpen(true);
  };

  const handleSave = (values: Record<string, unknown>) => {
    generateMutation.mutate(values);
    setDialogOpen(false);
  };

  const extraActions = (row: ListRow) => (
    <Button
      size="small"
      component={Link}
      href={`/grocery-lists/${row.groceryListID as number}`}
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
      <CrudDialog
        open={dialogOpen}
        title="Generate Grocery List"
        fields={generateFields}
        values={dialogData}
        error={generateMutation.error as Error | null}
        onClose={() => setDialogOpen(false)}
        onSave={handleSave}
      />
    </Box>
  );
}
