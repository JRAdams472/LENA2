"use client";

import { Fragment, use, useEffect, useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import Alert from "@mui/material/Alert";
import Autocomplete from "@mui/material/Autocomplete";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Divider from "@mui/material/Divider";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
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
import { api, EventRecipeStepInput, EventRecipeItemInput } from "@/lib/api";
import { EventRecipe, EventRecipeItem, EventRecipeStep, EventTimelineRecipe, Item } from "@/lib/types";

const MEAL_TYPES = ["breakfast", "lunch", "dinner", "snack", "other"];
const STEP_TYPES = ["prep", "cook", "rest", "wait", "serve", "other"];

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

interface StepForm {
  eventRecipeStepID: number | null;
  eventRecipeID: number;
  instruction: string;
  durationMinutes: string;
  stepType: string;
  isPassive: boolean;
  appliance: string;
}

interface ItemForm {
  eventRecipeItemID: number | null;
  eventRecipeID: number;
  // itemID is preserved from the row being edited; item is set when the
  // user picks a different one in the autocomplete.
  itemID: number;
  item: Item | null;
  quantity: string;
  unit: string;
  isOptional: boolean;
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
  const [stepDialogOpen, setStepDialogOpen] = useState(false);
  const [stepForm, setStepForm] = useState<StepForm | null>(null);
  const [itemDialogOpen, setItemDialogOpen] = useState(false);
  const [itemForm, setItemForm] = useState<ItemForm | null>(null);
  const [itemSearch, setItemSearch] = useState("");
  const [debouncedItemSearch, setDebouncedItemSearch] = useState("");
  const [expandedSlot, setExpandedSlot] = useState<number | null>(null);
  const [showTimeline, setShowTimeline] = useState(false);

  const eventQuery = useQuery({
    queryKey: ["foodEvent", eventId],
    queryFn: () => api.getFoodEvent(eventId),
    enabled: !isNaN(eventId),
  });

  const timelineQuery = useQuery({
    queryKey: ["eventTimeline", eventId],
    queryFn: () => api.getEventTimeline(eventId),
    enabled: !isNaN(eventId) && showTimeline,
  });

  const recipesQuery = useQuery({
    queryKey: ["recipes"],
    queryFn: () => api.getRecipes(),
  });

  useEffect(() => {
    const t = setTimeout(() => setDebouncedItemSearch(itemSearch), 300);
    return () => clearTimeout(t);
  }, [itemSearch]);

  const itemSearchQuery = useQuery({
    queryKey: ["items-search", debouncedItemSearch],
    queryFn: () => api.searchItems(debouncedItemSearch),
    enabled: debouncedItemSearch.length >= 2,
  });

  const event = eventQuery.data;
  const granularity = event?.slotGranularityMinutes ?? 15;
  const slots = useMemo(() => timeOptions(granularity), [granularity]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["foodEvent", eventId] });
    queryClient.invalidateQueries({ queryKey: ["eventTimeline", eventId] });
  };

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

  const saveStepMutation = useMutation({
    mutationFn: (f: StepForm) => {
      const payload: EventRecipeStepInput = {
        instruction: f.instruction,
        durationMinutes: f.durationMinutes === "" ? null : Number(f.durationMinutes),
        stepType: f.stepType === "" ? null : f.stepType,
        isPassive: f.isPassive,
        appliance: f.appliance === "" ? null : f.appliance,
      };
      return f.eventRecipeStepID === null
        ? api.addEventRecipeStep(f.eventRecipeID, payload)
        : api.updateEventRecipeStep(f.eventRecipeStepID, payload);
    },
    onSuccess: () => {
      invalidate();
      setStepDialogOpen(false);
    },
  });

  const removeStepMutation = useMutation({
    mutationFn: (stepId: number) => api.removeEventRecipeStep(stepId),
    onSuccess: invalidate,
  });

  const saveItemMutation = useMutation({
    mutationFn: (f: ItemForm) => {
      const payload: EventRecipeItemInput = {
        itemID: f.item?.itemID ?? f.itemID,
        quantity: Number(f.quantity),
        unit: f.unit,
        isOptional: f.isOptional,
        notes: f.notes === "" ? null : f.notes,
      };
      return f.eventRecipeItemID === null
        ? api.addEventRecipeItem(f.eventRecipeID, payload)
        : api.updateEventRecipeItem(f.eventRecipeItemID, payload);
    },
    onSuccess: () => {
      invalidate();
      setItemDialogOpen(false);
    },
  });

  const removeItemMutation = useMutation({
    mutationFn: (itemId: number) => api.removeEventRecipeItem(itemId),
    onSuccess: invalidate,
  });

  const syncStepsMutation = useMutation({
    mutationFn: (eventRecipeId: number) => api.syncEventRecipe(eventRecipeId),
    onSuccess: invalidate,
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

  const openStepCreate = (r: EventRecipe) => {
    setStepForm({
      eventRecipeStepID: null,
      eventRecipeID: r.eventRecipeID,
      instruction: "",
      durationMinutes: "",
      stepType: "",
      isPassive: false,
      appliance: "",
    });
    setStepDialogOpen(true);
  };

  const openStepEdit = (r: EventRecipe, s: EventRecipeStep) => {
    setStepForm({
      eventRecipeStepID: s.eventRecipeStepID,
      eventRecipeID: r.eventRecipeID,
      instruction: s.instruction,
      durationMinutes: s.durationMinutes != null ? String(s.durationMinutes) : "",
      stepType: s.stepType ?? "",
      isPassive: s.isPassive,
      appliance: s.appliance ?? "",
    });
    setStepDialogOpen(true);
  };

  const openItemCreate = (r: EventRecipe) => {
    setItemForm({
      eventRecipeItemID: null,
      eventRecipeID: r.eventRecipeID,
      itemID: 0,
      item: null,
      quantity: "",
      unit: "",
      isOptional: false,
      notes: "",
    });
    setItemSearch("");
    setDebouncedItemSearch("");
    setItemDialogOpen(true);
  };

  const openItemEdit = (r: EventRecipe, i: EventRecipeItem) => {
    setItemForm({
      eventRecipeItemID: i.eventRecipeItemID,
      eventRecipeID: r.eventRecipeID,
      itemID: i.itemID,
      item: null,
      quantity: String(i.baseQuantity),
      unit: i.unit,
      isOptional: i.isOptional,
      notes: i.notes ?? "",
    });
    setItemSearch(i.itemName ?? "");
    setDebouncedItemSearch("");
    setItemDialogOpen(true);
  };

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
                  <TableCell>Steps</TableCell>
                  <TableCell>Servings</TableCell>
                  <TableCell>Notes</TableCell>
                  <TableCell>Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {rows.map((r) => (
                  <Fragment key={r.eventRecipeID}>
                    <TableRow>
                      <TableCell>{hhmmOf(r.targetTime)}</TableCell>
                      <TableCell>{r.mealType}</TableCell>
                      <TableCell>{recipeName(r) ?? <em>Free-form</em>}</TableCell>
                      <TableCell>
                        <Button
                          size="small"
                          onClick={() =>
                            setExpandedSlot(
                              expandedSlot === r.eventRecipeID ? null : r.eventRecipeID
                            )
                          }
                        >
                          {(r.steps ?? []).length}
                          {expandedSlot === r.eventRecipeID ? " ▲" : " ▼"}
                        </Button>
                      </TableCell>
                      <TableCell>
                        {r.servings ?? "—"}
                        {r.scalingFactor !== 1 && r.baseServings != null && (
                          <Typography
                            component="span"
                            variant="caption"
                            color="text.secondary"
                            sx={{ display: "block" }}
                          >
                            ×{r.scalingFactor} of {r.baseServings}
                          </Typography>
                        )}
                      </TableCell>
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
                    {expandedSlot === r.eventRecipeID && (
                      <TableRow>
                        <TableCell colSpan={7} sx={{ backgroundColor: "action.hover" }}>
                          <SlotSteps
                            slot={r}
                            onAdd={() => openStepCreate(r)}
                            onEdit={(s) => openStepEdit(r, s)}
                            onRemove={(id) => removeStepMutation.mutate(id)}
                            onSync={() => {
                              if (
                                window.confirm(
                                  "Re-copy the linked recipe's steps and ingredients? Edits to this slot's copies will be lost."
                                )
                              ) {
                                syncStepsMutation.mutate(r.eventRecipeID);
                              }
                            }}
                          />
                          <Divider sx={{ my: 1 }} />
                          <SlotItems
                            slot={r}
                            onAdd={() => openItemCreate(r)}
                            onEdit={(i) => openItemEdit(r, i)}
                            onRemove={(id) => removeItemMutation.mutate(id)}
                          />
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Paper>

      <Paper sx={{ p: 3 }}>
        <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", mb: 1 }}>
          <Typography variant="h5">Timeline</Typography>
          <Button
            variant="outlined"
            onClick={() => setShowTimeline((v) => !v)}
          >
            {showTimeline ? "Hide Timeline" : "Generate Timeline"}
          </Button>
        </Box>
        {showTimeline && (
          <>
            {timelineQuery.isLoading && <CircularProgress size={24} />}
            {timelineQuery.error && (
              <Alert severity="error">{(timelineQuery.error as Error).message}</Alert>
            )}
            {timelineQuery.data && (
              <>
                {timelineQuery.data.warnings.map((w) => (
                  <Alert severity="warning" key={w} sx={{ mb: 1 }}>
                    {w}
                  </Alert>
                ))}
                {timelineQuery.data.recipes.length === 0 ? (
                  <Typography color="text.secondary">
                    No dishes to schedule yet.
                  </Typography>
                ) : (
                  timelineQuery.data.recipes.map((r) => (
                    <TimelineRecipeCard key={r.eventRecipeID} recipe={r} />
                  ))
                )}
              </>
            )}
          </>
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

      <Dialog open={stepDialogOpen} onClose={() => setStepDialogOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>
          {stepForm?.eventRecipeStepID === null ? "Add Step" : "Edit Step"}
        </DialogTitle>
        <DialogContent>
          {stepForm && (
            <>
              <TextField
                label="Instruction"
                fullWidth
                margin="dense"
                value={stepForm.instruction}
                onChange={(e) => setStepForm({ ...stepForm, instruction: e.target.value })}
              />
              <TextField
                label="Duration (minutes)"
                type="number"
                fullWidth
                margin="dense"
                value={stepForm.durationMinutes}
                onChange={(e) => setStepForm({ ...stepForm, durationMinutes: e.target.value })}
              />
              <FormControl fullWidth margin="dense">
                <InputLabel id="step-type-label">Type</InputLabel>
                <Select
                  labelId="step-type-label"
                  label="Type"
                  value={stepForm.stepType}
                  onChange={(e) => setStepForm({ ...stepForm, stepType: e.target.value })}
                >
                  <MenuItem value="">
                    <em>None</em>
                  </MenuItem>
                  {STEP_TYPES.map((t) => (
                    <MenuItem key={t} value={t}>
                      {t}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <TextField
                label="Appliance"
                fullWidth
                margin="dense"
                value={stepForm.appliance}
                onChange={(e) => setStepForm({ ...stepForm, appliance: e.target.value })}
              />
              <FormControlLabel
                control={
                  <Checkbox
                    checked={stepForm.isPassive}
                    onChange={(e) => setStepForm({ ...stepForm, isPassive: e.target.checked })}
                  />
                }
                label="Hands-off (runs unattended)"
              />
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setStepDialogOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            onClick={() => stepForm && saveStepMutation.mutate(stepForm)}
            disabled={saveStepMutation.isPending || stepForm?.instruction.trim() === ""}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={itemDialogOpen} onClose={() => setItemDialogOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>
          {itemForm?.eventRecipeItemID === null ? "Add Ingredient" : "Edit Ingredient"}
        </DialogTitle>
        <DialogContent>
          {itemForm && (
            <>
              <Autocomplete
                options={itemSearchQuery.data ?? []}
                getOptionLabel={(item) => item?.name ?? ""}
                isOptionEqualToValue={(option, value) => option?.itemID === value?.itemID}
                inputValue={itemSearch}
                onInputChange={(_, value) => setItemSearch(value)}
                value={itemForm.item}
                onChange={(_, value) => {
                  setItemForm({ ...itemForm, item: value });
                  if (value) void api.recordSelection("item", value.itemID);
                }}
                filterOptions={(options) => options}
                loading={itemSearchQuery.isLoading}
                noOptionsText={
                  debouncedItemSearch.length < 2 ? "Type at least 2 characters" : "No items found"
                }
                renderInput={(params) => (
                  <TextField {...params} label="Item" margin="dense" />
                )}
              />
              <TextField
                label="Quantity (per recipe serving)"
                type="number"
                fullWidth
                margin="dense"
                value={itemForm.quantity}
                onChange={(e) => setItemForm({ ...itemForm, quantity: e.target.value })}
                helperText="The base amount — it is multiplied by the slot's servings ÷ recipe servings when displayed."
              />
              <TextField
                label="Unit"
                fullWidth
                margin="dense"
                value={itemForm.unit}
                onChange={(e) => setItemForm({ ...itemForm, unit: e.target.value })}
                helperText='Name or abbreviation, e.g. "cup", "g", "each"'
              />
              <TextField
                label="Notes"
                fullWidth
                margin="dense"
                value={itemForm.notes}
                onChange={(e) => setItemForm({ ...itemForm, notes: e.target.value })}
              />
              <FormControlLabel
                control={
                  <Checkbox
                    checked={itemForm.isOptional}
                    onChange={(e) => setItemForm({ ...itemForm, isOptional: e.target.checked })}
                  />
                }
                label="Optional"
              />
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setItemDialogOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            onClick={() => itemForm && saveItemMutation.mutate(itemForm)}
            disabled={
              saveItemMutation.isPending ||
              (itemForm?.item?.itemID ?? itemForm?.itemID ?? 0) === 0 ||
              itemForm?.quantity.trim() === "" ||
              itemForm?.unit.trim() === ""
            }
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>
      <Divider />
    </Box>
  );
}

// SlotSteps lists a slot's snapshot steps with per-event edit actions.
// These rows live on the event slot — edits never reach the shared recipe.
function SlotSteps({
  slot,
  onAdd,
  onEdit,
  onRemove,
  onSync,
}: {
  slot: EventRecipe;
  onAdd: () => void;
  onEdit: (step: EventRecipeStep) => void;
  onRemove: (stepId: number) => void;
  onSync: () => void;
}) {
  const steps = slot.steps ?? [];
  return (
    <Box sx={{ py: 1 }}>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", mb: 1 }}>
        <Typography variant="subtitle2">
          Event-specific steps{slot.recipeID ? " (copied from recipe — edits stay on this event)" : ""}
        </Typography>
        <Box sx={{ display: "flex", gap: 1 }}>
          {slot.recipeID != null && (
            <Button size="small" onClick={onSync}>
              Sync from recipe
            </Button>
          )}
          <Button size="small" variant="outlined" onClick={onAdd}>
            Add step
          </Button>
        </Box>
      </Box>
      {steps.length === 0 ? (
        <Typography color="text.secondary" variant="body2">
          No steps yet — add steps to include this dish in the timeline.
        </Typography>
      ) : (
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>#</TableCell>
              <TableCell>Instruction</TableCell>
              <TableCell>Duration</TableCell>
              <TableCell>Type</TableCell>
              <TableCell>Appliance</TableCell>
              <TableCell>Flags</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {steps.map((s) => (
              <TableRow key={s.eventRecipeStepID}>
                <TableCell>{s.stepNumber}</TableCell>
                <TableCell>{s.instruction}</TableCell>
                <TableCell>
                  {s.durationMinutes != null ? `${s.durationMinutes} min` : "—"}
                </TableCell>
                <TableCell>{s.stepType ?? "—"}</TableCell>
                <TableCell>{s.appliance ?? "—"}</TableCell>
                <TableCell>{s.isPassive ? "hands-off" : ""}</TableCell>
                <TableCell>
                  <IconButton size="small" aria-label="Edit step" onClick={() => onEdit(s)}>
                    <EditIcon fontSize="small" />
                  </IconButton>
                  <IconButton
                    size="small"
                    aria-label="Delete step"
                    onClick={() => onRemove(s.eventRecipeStepID)}
                  >
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Box>
  );
}

// SlotItems lists a slot's ingredient snapshot with quantities scaled by
// the slot's servings ÷ the recipe's frozen base servings. These rows
// live on the event slot — edits never reach the shared recipe.
function SlotItems({
  slot,
  onAdd,
  onEdit,
  onRemove,
}: {
  slot: EventRecipe;
  onAdd: () => void;
  onEdit: (item: EventRecipeItem) => void;
  onRemove: (itemId: number) => void;
}) {
  const items = [...(slot.items ?? [])].sort((a, b) => a.displayOrder - b.displayOrder);
  return (
    <Box sx={{ py: 1 }}>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", mb: 1 }}>
        <Typography variant="subtitle2">
          Ingredients
          {slot.baseServings != null
            ? ` — scaled for ${slot.servings ?? "—"} servings (recipe makes ${slot.baseServings})`
            : slot.recipeID
            ? " (copied from recipe — edits stay on this event)"
            : ""}
        </Typography>
        <Button size="small" variant="outlined" onClick={onAdd}>
          Add ingredient
        </Button>
      </Box>
      {items.length === 0 ? (
        <Typography color="text.secondary" variant="body2">
          No ingredients yet.
        </Typography>
      ) : (
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Item</TableCell>
              <TableCell align="right">Qty</TableCell>
              <TableCell>Unit</TableCell>
              <TableCell>Section</TableCell>
              <TableCell>Notes</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {items.map((i) => (
              <TableRow key={i.eventRecipeItemID}>
                <TableCell>
                  {i.itemName ?? `Item ${i.itemID}`}
                  {i.isOptional && (
                    <Typography component="span" variant="caption" color="text.secondary">
                      {" "}(optional)
                    </Typography>
                  )}
                </TableCell>
                <TableCell align="right">
                  {i.quantity}
                  {i.quantity !== i.baseQuantity && (
                    <Typography
                      component="span"
                      variant="caption"
                      color="text.secondary"
                      sx={{ display: "block" }}
                    >
                      {i.baseQuantity} base
                    </Typography>
                  )}
                </TableCell>
                <TableCell>{i.unit}</TableCell>
                <TableCell>{i.section ?? "—"}</TableCell>
                <TableCell>{i.notes ?? ""}</TableCell>
                <TableCell>
                  <IconButton size="small" aria-label="Edit ingredient" onClick={() => onEdit(i)}>
                    <EditIcon fontSize="small" />
                  </IconButton>
                  <IconButton
                    size="small"
                    aria-label="Delete ingredient"
                    onClick={() => onRemove(i.eventRecipeItemID)}
                  >
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Box>
  );
}

function TimelineRecipeCard({ recipe }: { recipe: EventTimelineRecipe }) {
  return (
    <Box sx={{ mb: 2 }}>
      <Typography variant="h6">
        {recipe.name}
        <Typography component="span" color="text.secondary" sx={{ ml: 1 }}>
          serve {hhmmOf(recipe.targetTime)}
          {recipe.startBy && ` · start by ${hhmmOf(recipe.startBy)}`}
        </Typography>
      </Typography>
      {recipe.warnings.map((w) => (
        <Alert severity="warning" key={w} sx={{ my: 1 }}>
          {w}
        </Alert>
      ))}
      {recipe.unschedulable ? (
        <Typography color="text.secondary">
          This dish cannot be scheduled (no recipe steps).
        </Typography>
      ) : (
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Start</TableCell>
                <TableCell>End</TableCell>
                <TableCell>Step</TableCell>
                <TableCell>Appliance</TableCell>
                <TableCell>Flags</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {recipe.steps.map((s) => (
                <TableRow
                  key={s.stepNumber}
                  sx={s.conflicts.length > 0 ? { backgroundColor: "error.light" } : undefined}
                >
                  <TableCell>{hhmmOf(s.startTime)}</TableCell>
                  <TableCell>{hhmmOf(s.endTime)}</TableCell>
                  <TableCell>
                    {s.stepNumber}. {s.instruction}
                    {s.stepType && (
                      <Typography component="span" color="text.secondary">
                        {" "}({s.stepType})
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell>{s.appliance ?? "—"}</TableCell>
                  <TableCell>
                    {s.estimated && "est. "}
                    {s.isPassive && "hands-off "}
                    {s.conflicts.length > 0 && (
                      <Typography component="span" color="error">
                        conflict
                      </Typography>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Box>
  );
}
