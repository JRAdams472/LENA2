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

const WEEKDAYS = [
  "Sunday",
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
];

const planFields: FieldDef<MealPlan>[] = [
  { key: "planName", label: "Plan Name" },
  { key: "weekStartDate", label: "Week Start Date", type: "date" },
  { key: "weekStartDayOfWeek", label: "Week Start Day (0=Sun)", type: "number" },
  { key: "isActive", label: "Active", type: "boolean" },
];

type PlanRow = ReturnType<typeof toRow>;

// Server-side sort keys "planName"/"weekStartDate" match the row keys
// exactly — the weekday and flag columns aren't in the allowlist.
const planTableFields: FieldDef<PlanRow>[] = [
  { key: "planName", label: "Plan Name", sortable: true },
  { key: "weekStartDate", label: "Week Of", type: "date", sortable: true },
  {
    key: "weekStartDayOfWeek",
    label: "Week Starts On",
    sortable: false,
    render: (row) => WEEKDAYS[row.weekStartDayOfWeek] ?? "—",
  },
  { key: "isActive", label: "Active", type: "boolean", sortable: false },
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
  // Server-side sort state — null keeps the week-descending default.
  const [sort, setSort] = useState<{ key: string; desc: boolean } | null>(null);

  const listQuery = useQuery({
    queryKey: ["mealPlans", pageNumber, pageSize, sort],
    queryFn: () => api.getMealPlansPaged(pageNumber, pageSize, sort ?? undefined),
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
    createMutation.reset();
    updateMutation.reset();
    setIsCreate(true);
    setDialogData({ weekStartDayOfWeek: 0, isActive: true });
    setDialogOpen(true);
  };

  const handleEdit = (row: Record<string, unknown>) => {
    createMutation.reset();
    updateMutation.reset();
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
    const mutation = isCreate ? createMutation : updateMutation;
    mutation.mutate(values, { onSuccess: () => setDialogOpen(false) });
  };

  const extraActions = (row: Record<string, unknown>) => (
    <Button
      size="small"
      component={Link}
      href={`/member/meal-plans/${row.mealPlanID as number}`}
    >
      Manage
    </Button>
  );

  return (
    <Box>
      <DataTable
        title="Meal Plans"
        rows={(listQuery.data?.items ?? []).map(toRow)}
        fields={planTableFields}
        isLoading={listQuery.isLoading}
        error={(listQuery.error ?? deleteMutation.error) as Error | null}
        onCreate={handleCreate}
        onEdit={handleEdit}
        onDelete={handleDelete}
        extraActions={extraActions}
        serverSort={(key, desc) => {
          setSort({ key, desc });
          setPageNumber(1);
        }}
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
        error={createMutation.error ?? updateMutation.error}
        saving={createMutation.isPending || updateMutation.isPending}
        onClose={() => setDialogOpen(false)}
        onSave={handleSave}
      />
    </Box>
  );
}
