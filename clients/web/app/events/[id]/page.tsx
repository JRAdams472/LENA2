"use client";

import { use, useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Divider from "@mui/material/Divider";
import FormControl from "@mui/material/FormControl";
import IconButton from "@mui/material/IconButton";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Paper from "@mui/material/Paper";
import Select from "@mui/material/Select";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import { api } from "@/lib/api";
import { EventRecipe } from "@/lib/types";

const MEAL_TYPES = ["breakfast", "lunch", "dinner", "snack", "other"];

// Generate "HH:mm" options at the event's slot granularity.
function timeOptions(granularity: number): string[] {
  const out: string[] = [];
  for (let mins = 0; mins < 24 * 60; mins += granularity) {
    const h = String(Math.floor(mins / 60)).padStart(2, "0");
    const m = String(mins % 60).padStart(2, "0");
    out.push(`${h}:${m}`);
  }
  return out;
}

// The resolver compares targetTime's date to the event date using the
// timestamp's own offset, so emit UTC to keep the two aligned.
function toTargetTime(eventDate: string, hhmm: string): string {
  return `${eventDate}T${hhmm}:00Z`;
}

function hhmmOf(iso: string): string {
  const d = new Date(iso);
  return `${String(d.getUTCHours()).padStart(2, "0")}:${String(d.getUTCMinutes()).padStart(2, "0")}`;
}

interface SlotForm {
  eventRecipeID: number | null;
  recipeID: number | null;
  mealType: string;
  time: string;
  servings: string;
  notes: string;
}

export default function EventDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const eventId = Number(id);
  const router = useRouter();
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [form, setForm] = useState<SlotForm | null>(null);

  const eventQuery = useQuery({
    queryKey: ["foodEvent", eventId],
    queryFn: () => api.getFoodEvent(eventId),
    enabled: !isNaN(eventId),
  });

  const recipesQuery = useQuery({
    queryKey: ["recipes"],
    queryFn: () => api.getRecipes(),
  });

  const event = eventQuery.data;
  const granularity = event?.slotGranularityMinutes ?? 15;
  const slots = useMemo(() => timeOptions(granularity), [granularity]);

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["foodEvent", eventId] });

  const saveSlotMutation = useMutation({
    mutationFn: (f: SlotForm) => {
      const payload = {
        recipeID: f.recipeID,
        mealType: f.mealType,
        targetTime: toTargetTime(event?.eventDate ?? "", f.time),
        servings: f.servings === "" ? null : Number(f.servings),
        notes: f.notes === "" ? null : f.notes,
      };
      return f.eventRecipeID === null
        ? api.addEventRecipe(eventId, payload)
        : api.updateEventRecipe(f.eventRecipeID, payload);
    },
    onSuccess: () => {
      invalidate();
      setDialogOpen(false);
    },
  });

  const removeSlotMutation = useMutation({
    mutationFn: (slotId: number) => api.removeEventRecipe(slotId),
    onSuccess: invalidate,
  });

  const deleteEventMutation = useMutation({
    mutationFn: () => api.deleteFoodEvent(eventId),
    onSuccess: () => router.push("/events"),
  });

  const openCreate = () => {
    setForm({
      eventRecipeID: null,
      recipeID: null,
      mealType: "dinner",
      time: slots[0] ?? "18:00",
      servings: "",
      notes: "",
    });
    setDialogOpen(true);
  };

  const openEdit = (r: EventRecipe) => {
    setForm({
      eventRecipeID: r.eventRecipeID,
      recipeID: r.recipeID,
      mealType: r.mealType,
      time: hhmmOf(r.targetTime),
      servings: r.servings != null ? String(r.servings) : "",
      notes: r.notes ?? "",
    });
    setDialogOpen(true);
  };

  const recipeName = (r: EventRecipe) =>
    r.recipe?.recipeName ?? (r.recipeID ? `Recipe ${r.recipeID}` : null);

  if (isNaN(eventId)) {
    return <Alert severity="error">Invalid event id</Alert>;
  }
  if (eventQuery.isLoading) return <CircularProgress />;
  if (eventQuery.error) {
    return <Alert severity="error">{(eventQuery.error as Error).message}</Alert>;
  }
  if (!event) return <Alert severity="error">Event not found</Alert>;

  const rows = [...(event.eventRecipes ?? [])].sort((a, b) =>
    a.targetTime.localeCompare(b.targetTime)
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Paper sx={{ p: 3 }}>
        <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
          <Box>
            <Typography variant="h4" gutterBottom>
              {event.name}
            </Typography>
            <Typography color="text.secondary">
              {event.eventDate} · {event.slotGranularityMinutes}-minute slots
              {!event.isActive && " · inactive"}
            </Typography>
          </Box>
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button variant="contained" onClick={openCreate}>
              Add Dish
            </Button>
            <Button
              variant="outlined"
              color="error"
              onClick={() => {
                if (window.confirm(`Delete "${event.name}"?`)) {
                  deleteEventMutation.mutate();
                }
              }}
              disabled={deleteEventMutation.isPending}
            >
              Delete Event
            </Button>
          </Box>
        </Box>
        {(saveSlotMutation.error || removeSlotMutation.error || deleteEventMutation.error) && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {
              ((saveSlotMutation.error ??
                removeSlotMutation.error ??
                deleteEventMutation.error) as Error).message
            }
          </Alert>
        )}
      </Paper>

      <Paper sx={{ p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Dishes
        </Typography>
        {rows.length === 0 ? (
          <Typography color="text.secondary">
            No dishes yet — add a recipe or a free-form slot.
          </Typography>
        ) : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Serve At</TableCell>
                  <TableCell>Meal</TableCell>
                  <TableCell>Recipe</TableCell>
                  <TableCell>Servings</TableCell>
                  <TableCell>Notes</TableCell>
                  <TableCell>Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {rows.map((r) => (
                  <TableRow key={r.eventRecipeID}>
                    <TableCell>{hhmmOf(r.targetTime)}</TableCell>
                    <TableCell>{r.mealType}</TableCell>
                    <TableCell>{recipeName(r) ?? <em>Free-form</em>}</TableCell>
                    <TableCell>{r.servings ?? "—"}</TableCell>
                    <TableCell>{r.notes ?? ""}</TableCell>
                    <TableCell>
                      <IconButton size="small" aria-label="Edit" onClick={() => openEdit(r)}>
                        <EditIcon fontSize="small" />
                      </IconButton>
                      <IconButton
                        size="small"
                        aria-label="Delete"
                        onClick={() => removeSlotMutation.mutate(r.eventRecipeID)}
                      >
                        <DeleteIcon fontSize="small" />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Paper>

      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>
          {form?.eventRecipeID === null ? "Add Dish" : "Edit Dish"}
        </DialogTitle>
        <DialogContent>
          {form && (
            <>
              <FormControl fullWidth margin="dense">
                <InputLabel id="slot-recipe-label">Recipe</InputLabel>
                <Select
                  labelId="slot-recipe-label"
                  label="Recipe"
                  value={form.recipeID === null ? "" : String(form.recipeID)}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      recipeID: e.target.value === "" ? null : Number(e.target.value),
                    })
                  }
                >
                  <MenuItem value="">
                    <em>Free-form (no recipe)</em>
                  </MenuItem>
                  {(recipesQuery.data ?? []).map((r) => (
                    <MenuItem key={r.recipeID} value={String(r.recipeID)}>
                      {r.recipeName}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl fullWidth margin="dense">
                <InputLabel id="slot-meal-label">Meal</InputLabel>
                <Select
                  labelId="slot-meal-label"
                  label="Meal"
                  value={form.mealType}
                  onChange={(e) => setForm({ ...form, mealType: e.target.value })}
                >
                  {MEAL_TYPES.map((mt) => (
                    <MenuItem key={mt} value={mt}>
                      {mt}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl fullWidth margin="dense">
                <InputLabel id="slot-time-label">Serve At</InputLabel>
                <Select
                  labelId="slot-time-label"
                  label="Serve At"
                  value={form.time}
                  onChange={(e) => setForm({ ...form, time: e.target.value })}
                >
                  {slots.map((t) => (
                    <MenuItem key={t} value={t}>
                      {t}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <TextField
                label="Servings"
                type="number"
                fullWidth
                margin="dense"
                value={form.servings}
                onChange={(e) => setForm({ ...form, servings: e.target.value })}
              />
              <TextField
                label="Notes"
                fullWidth
                margin="dense"
                value={form.notes}
                onChange={(e) => setForm({ ...form, notes: e.target.value })}
              />
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            onClick={() => form && saveSlotMutation.mutate(form)}
            disabled={saveSlotMutation.isPending || form?.time === ""}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>
      <Divider />
    </Box>
  );
}
