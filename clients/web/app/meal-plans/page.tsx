"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Link from "next/link";
import { api, asEntity } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";
import { MealPlan } from "@/lib/types";

const planFields: FieldDef<MealPlan>[] = [
  { key: "planName", label: "Plan Name" },
  { key: "weekStartDate", label: "Week Start Date", type: "date" },
  { key: "weekStartDayOfWeek", label: "Week Start Day (0=Sun)", type: "number" },
  { key: "isActive", label: "Active", type: "boolean" },
];

function toRow(plan: MealPlan) {
  return {
    mealPlanID: plan.mealPlanID,
    planName: plan.planName,
    weekStartDate: plan.weekStartDate?.split("T")[0] ?? "",
    weekStartDayOfWeek: plan.weekStartDayOfWeek,
    isActive: plan.isActive,
  };
}

export default function MealPlansPage() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogData, setDialogData] = useState<Record<string, unknown>>({});
  const [isCreate, setIsCreate] = useState(false);
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(25);

  const listQuery = useQuery({
    queryKey: ["mealPlans", pageNumber, pageSize],
    queryFn: () => api.getMealPlansPaged(pageNumber, pageSize),
    placeholderData: (prev) => prev,
  });

  const createMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.createMealPlan(asEntity(row)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["mealPlans"] }),
  });

  const updateMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.updateMealPlan(row.mealPlanID as number, asEntity(row)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["mealPlans"] }),
  });

  const deleteMutation = useMutation({
    mutationFn: (row: Record<string, unknown>) =>
      api.deleteMealPlan(row.mealPlanID as number),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["mealPlans"] }),
  });

  const handleCreate = () => {
    setIsCreate(true);
    setDialogData({ weekStartDayOfWeek: 0, isActive: true });
    setDialogOpen(true);
  };

  const handleEdit = (row: Record<string, unknown>) => {
    setIsCreate(false);
    setDialogData({ ...row });
    setDialogOpen(true);
  };

  const handleDelete = (row: Record<string, unknown>) => {
    if (window.confirm("Delete this meal plan?")) {
      deleteMutation.mutate(row);
    }
  };

  const handleSave = (values: Record<string, unknown>) => {
    if (isCreate) {
      createMutation.mutate(values);
    } else {
      updateMutation.mutate(values);
    }
    setDialogOpen(false);
  };

  const extraActions = (row: Record<string, unknown>) => (
    <Button
      size="small"
      component={Link}
      href={`/meal-plans/${row.mealPlanID as number}`}
    >
      Manage
    </Button>
  );

  return (
    <Box>
      <DataTable
        title="Meal Plans"
        rows={(listQuery.data?.items ?? []).map(toRow)}
        isLoading={listQuery.isLoading}
        error={listQuery.error as Error | null}
        onCreate={handleCreate}
        onEdit={handleEdit}
        onDelete={handleDelete}
        extraActions={extraActions}
        pagination={
          listQuery.data
            ? {
                pageNumber,
                pageSize,
                totalCount: listQuery.data.totalCount,
                totalPages: listQuery.data.totalPages,
                onPageChange: setPageNumber,
                onPageSizeChange: (size) => { setPageSize(size); setPageNumber(1); },
              }
            : undefined
        }
      />
      <CrudDialog
        open={dialogOpen}
        title={isCreate ? "Create Meal Plan" : "Edit Meal Plan"}
        fields={planFields}
        values={dialogData}
        onClose={() => setDialogOpen(false)}
        onSave={handleSave}
      />
    </Box>
  );
}
