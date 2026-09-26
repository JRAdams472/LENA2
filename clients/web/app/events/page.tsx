"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Alert from "@mui/material/Alert";
import TextField from "@mui/material/TextField";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import Select from "@mui/material/Select";
import MenuItem from "@mui/material/MenuItem";
import FormControlLabel from "@mui/material/FormControlLabel";
import Switch from "@mui/material/Switch";
import Link from "next/link";
import { api } from "@/lib/api";
import DataTable from "@/app/components/DataTable";
import { FoodEvent } from "@/lib/types";

interface EventFormState {
  foodEventID: number | null;
  name: string;
  eventDate: string;
  slotGranularityMinutes: number;
  isActive: boolean;
}

const emptyForm: EventFormState = {
  foodEventID: null,
  name: "",
  eventDate: "",
  slotGranularityMinutes: 15,
  isActive: true,
};

function toRow(ev: FoodEvent) {
  return {
    foodEventID: ev.foodEventID,
    name: ev.name,
    eventDate: ev.eventDate,
    slotGranularityMinutes: ev.slotGranularityMinutes,
    isActive: ev.isActive,
  };
}

export default function EventsPage() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [form, setForm] = useState<EventFormState>(emptyForm);
  const [pageNumber, setPageNumber] = useState(1);
  const [pageSize, setPageSize] = useState(25);

  const listQuery = useQuery({
    queryKey: ["foodEvents", pageNumber, pageSize],
    queryFn: () => api.getFoodEventsPaged(pageNumber, pageSize),
    placeholderData: (prev) => prev,
  });

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["foodEvents"] });

  const saveMutation = useMutation({
    mutationFn: (f: EventFormState) =>
      f.foodEventID === null
        ? api.createFoodEvent({
            name: f.name,
            eventDate: f.eventDate,
            slotGranularityMinutes: f.slotGranularityMinutes,
          })
        : api.updateFoodEvent(f.foodEventID, {
            name: f.name,
            eventDate: f.eventDate,
            slotGranularityMinutes: f.slotGranularityMinutes,
            isActive: f.isActive,
          }),
    onSuccess: () => {
      invalidate();
      setDialogOpen(false);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteFoodEvent(id),
    onSuccess: invalidate,
  });

  const handleCreate = () => {
    setForm(emptyForm);
    setDialogOpen(true);
  };

  const handleEdit = (row: Record<string, unknown>) => {
    setForm({
      foodEventID: row.foodEventID as number,
      name: (row.name as string) ?? "",
      eventDate: (row.eventDate as string) ?? "",
      slotGranularityMinutes: (row.slotGranularityMinutes as number) ?? 15,
      isActive: Boolean(row.isActive),
    });
    setDialogOpen(true);
  };

  const handleDelete = (row: Record<string, unknown>) => {
    if (window.confirm("Delete this event?")) {
      deleteMutation.mutate(row.foodEventID as number);
    }
  };

  const handleSave = () => {
    if (form.name.trim() === "" || form.eventDate === "") return;
    saveMutation.mutate(form);
  };

  const extraActions = (row: Record<string, unknown>) => (
    <Button
      size="small"
      component={Link}
      href={`/events/${row.foodEventID as number}`}
    >
      Manage
    </Button>
  );

  return (
    <Box>
      <DataTable
        title="Food Events"
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
      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>
          {form.foodEventID === null ? "Create Event" : "Edit Event"}
        </DialogTitle>
        <DialogContent>
          {saveMutation.error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {(saveMutation.error as Error).message}
            </Alert>
          )}
          <TextField
            label="Name"
            fullWidth
            margin="dense"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
          />
          <TextField
            label="Event Date"
            type="date"
            fullWidth
            margin="dense"
            value={form.eventDate}
            onChange={(e) => setForm({ ...form, eventDate: e.target.value })}
            slotProps={{ inputLabel: { shrink: true } }}
          />
          <FormControl fullWidth margin="dense">
            <InputLabel id="granularity-label">Time Slot Granularity</InputLabel>
            <Select
              labelId="granularity-label"
              label="Time Slot Granularity"
              value={form.slotGranularityMinutes}
              onChange={(e) =>
                setForm({ ...form, slotGranularityMinutes: Number(e.target.value) })
              }
            >
              <MenuItem value={15}>15 minutes</MenuItem>
              <MenuItem value={30}>30 minutes</MenuItem>
            </Select>
          </FormControl>
          {form.foodEventID !== null && (
            <FormControlLabel
              control={
                <Switch
                  checked={form.isActive}
                  onChange={(e) => setForm({ ...form, isActive: e.target.checked })}
                />
              }
              label="Active"
              sx={{ display: "flex", mt: 1 }}
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>Cancel</Button>
          <Button
            onClick={handleSave}
            variant="contained"
            disabled={
              form.name.trim() === "" ||
              form.eventDate === "" ||
              saveMutation.isPending
            }
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
