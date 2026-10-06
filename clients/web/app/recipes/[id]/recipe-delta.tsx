"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Autocomplete from "@mui/material/Autocomplete";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import Chip from "@mui/material/Chip";
import Divider from "@mui/material/Divider";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
import IconButton from "@mui/material/IconButton";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Paper from "@mui/material/Paper";
import Radio from "@mui/material/Radio";
import RadioGroup from "@mui/material/RadioGroup";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import CloseIcon from "@mui/icons-material/Close";
import TuneIcon from "@mui/icons-material/Tune";
import { api } from "@/lib/api";
import {
  DeltaDraft,
  ItemDraft,
  StepDraft,
  describeItemChange,
  describeStepChange,
  draftFromDelta,
  draftIsDirty,
  itemDraftInputs,
  removeDraftRow,
  stepDraftInputs,
} from "@/lib/delta";
import { brandSuffix } from "@/lib/format";
import {
  Ingredient,
  Item,
  RecipeDelta,
  RecipeDeltaEvent,
  RecipeItem,
  RecipeStep,
  Unit,
} from "@/lib/types";
import IngredientAutocomplete from "@/app/components/IngredientAutocomplete";

let draftSeq = 0;
const nextKey = () => `new-${++draftSeq}`;

const numOrBlank = (v: number | null | undefined): string =>
  v != null ? String(v) : "";

const itemKindChip: Record<ItemDraft["kind"], string> = {
  substitute: "Swap",
  adjust: "Adjust",
  remove: "Remove",
  add: "Add",
};

const stepKindChip: Record<StepDraft["kind"], string> = {
  replace: "Edit",
  remove: "Remove",
  add: "Add",
};

const eventKindChip: Record<RecipeDeltaEvent["event"], string> = {
  set: "Saved",
  clear: "Cleared",
  acknowledge: "Reviewed",
};

/** One history line — "e2e@example.com saved the tweak set — 4 lines + 2 steps". */
function describeDeltaEvent(e: RecipeDeltaEvent): string {
  const tweakBits = [
    e.itemCount > 0
      ? `${e.itemCount} line${e.itemCount === 1 ? "" : "s"}`
      : null,
    e.stepCount > 0
      ? `${e.stepCount} step${e.stepCount === 1 ? "" : "s"}`
      : null,
  ].filter(Boolean);
  const tweaks = tweakBits.length > 0 ? ` — ${tweakBits.join(" + ")}` : "";
  switch (e.event) {
    case "set":
      return `${e.actor} saved the tweak set${tweaks}`;
    case "clear":
      return `${e.actor} cleared all tweaks${tweaks}`;
    case "acknowledge":
      return `${e.actor} marked the tweaks reviewed`;
    default:
      return `${e.actor}: ${e.event}`;
  }
}

/** Small filled chip marking a delta-produced row (Swapped/Added/…). */
export function DeltaBadge({ label }: { label: string }) {
  return (
    <Chip
      size="small"
      color="primary"
      label={label}
      sx={{ ml: 1, height: 20, fontSize: "0.7rem" }}
    />
  );
}

function useItemSearch() {
  const [text, setText] = useState("");
  const [debounced, setDebounced] = useState("");
  useEffect(() => {
    const t = setTimeout(() => setDebounced(text), 300);
    return () => clearTimeout(t);
  }, [text]);
  const query = useQuery({
    queryKey: ["items-search", debounced],
    queryFn: () => api.searchItems(debounced, undefined, 50),
    enabled: debounced.length >= 2,
  });
  return { text, setText, debounced, query };
}

// Brand search matching the recipe page's item autocomplete — picks a
// packaged item so a tweak can bind a specific brand.
function ItemPicker({
  value,
  onChange,
}: {
  value: Item | null;
  onChange: (item: Item | null) => void;
}) {
  const search = useItemSearch();
  return (
    <Autocomplete
      size="small"
      options={search.query.data ?? []}
      getOptionLabel={(item) => item?.name ?? ""}
      isOptionEqualToValue={(a, b) => a?.itemID === b?.itemID}
      inputValue={search.text}
      onInputChange={(_, v) => search.setText(v)}
      value={value}
      onChange={(_, v) => {
        onChange(v);
        if (v) void api.recordSelection("item", v.itemID);
      }}
      filterOptions={(options) => options}
      loading={search.query.isLoading}
      noOptionsText={
        search.debounced.length < 2 ? "Type at least 2 characters" : "No items found"
      }
      renderOption={(props, item) => {
        const { key, ...liProps } = props;
        return (
          <li key={item.itemID} {...liProps}>
            {item.name}
            {brandSuffix(item.brand, item.name)}
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField {...params} label="Brand item (optional)" size="small" />
      )}
      sx={{ minWidth: 240 }}
    />
  );
}

function UnitSelect({
  value,
  onChange,
  units,
}: {
  value: number | null;
  onChange: (id: number | null) => void;
  units: Unit[];
}) {
  return (
    <FormControl size="small" sx={{ minWidth: 110 }}>
      <InputLabel id="delta-unit-label">Unit</InputLabel>
      <Select
        labelId="delta-unit-label"
        label="Unit"
        value={value != null ? String(value) : ""}
        onChange={(e) =>
          onChange(e.target.value === "" ? null : Number(e.target.value))
        }
      >
        <MenuItem value="">
          <em>None</em>
        </MenuItem>
        {units.map((u) => (
          <MenuItem key={u.unitID} value={String(u.unitID)}>
            {u.name}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}

const STEP_TYPES = ["prep", "cook", "rest", "wait", "serve", "other"];

// Timing fields shared by the step replace editor and the add-step form.
function StepMetaFields({
  duration,
  setDuration,
  stepType,
  setStepType,
  passive,
  setPassive,
  appliance,
  setAppliance,
}: {
  duration: string;
  setDuration: (v: string) => void;
  stepType: string;
  setStepType: (v: string) => void;
  passive: boolean;
  setPassive: (v: boolean) => void;
  appliance: string;
  setAppliance: (v: string) => void;
}) {
  return (
    <>
      <TextField
        size="small"
        label="Duration (min)"
        type="number"
        value={duration}
        onChange={(e) => setDuration(e.target.value)}
        sx={{ width: 130 }}
      />
      <FormControl size="small" sx={{ minWidth: 120 }}>
        <InputLabel id="delta-step-type-label">Type</InputLabel>
        <Select
          labelId="delta-step-type-label"
          label="Type"
          value={stepType}
          onChange={(e) => setStepType(e.target.value)}
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
      <FormControlLabel
        control={
          <Checkbox
            checked={passive}
            onChange={(e) => setPassive(e.target.checked)}
          />
        }
        label="Passive"
      />
      <TextField
        size="small"
        label="Appliance"
        value={appliance}
        onChange={(e) => setAppliance(e.target.value)}
        sx={{ width: 130 }}
      />
    </>
  );
}

/**
 * Inline editor for one ingredient line — a substitute swaps the
 * ingredient/brand, an adjust patches amount/unit/optional, remove drops
 * the line from the household version.
 */
export function ItemTweakEditor({
  item,
  existing,
  units,
  onApply,
  onCancel,
}: {
  item: RecipeItem;
  existing: ItemDraft | null;
  units: Unit[];
  onApply: (row: ItemDraft) => void;
  onCancel: () => void;
}) {
  const [mode, setMode] = useState<"substitute" | "adjust" | "remove">(
    existing?.kind === "add" ? "adjust" : (existing?.kind ?? "substitute")
  );
  const [ingredient, setIngredient] = useState<Ingredient | null>(
    existing?.ingredientId
      ? ({ ingredientID: existing.ingredientId, name: existing.ingredientName } as Ingredient)
      : (item.ingredient ?? null)
  );
  const [item_, setItem] = useState<Item | null>(item.item ?? null);
  const [qty, setQty] = useState(
    existing?.quantity != null ? String(existing.quantity) : String(item.quantity)
  );
  const [unitId, setUnitId] = useState<number | null>(
    existing?.unitId ??
      units.find((u) => u.name === item.unitOfMeasure)?.unitID ??
      null
  );
  const [optional, setOptional] = useState<boolean>(
    existing?.isOptional ?? item.isOptional
  );
  const [notes, setNotes] = useState(existing?.notes ?? item.notes ?? "");

  const unitName = units.find((u) => u.unitID === unitId)?.name ?? null;

  const apply = () => {
    if (!item.recipeItemID) return;
    const anchor = { key: `line-${item.recipeItemID}`, recipeItemId: item.recipeItemID };
    if (mode === "remove") {
      onApply({
        ...anchor,
        kind: "remove",
        itemId: null,
        itemName: null,
        itemBrand: null,
        ingredientId: null,
        ingredientName: null,
        quantity: null,
        unitId: null,
        unitName: null,
        section: null,
        displayOrder: null,
        notes: null,
        isOptional: null,
        orphaned: false,
      });
      return;
    }
    if (mode === "substitute") {
      if (!ingredient && !item_) return;
      // Sparse: only the swapped refs change; canonical amounts flow
      // through unless the user also adjusts them here.
      onApply({
        ...anchor,
        kind: "substitute",
        itemId: item_?.itemID ?? null,
        itemName: item_?.name ?? null,
        itemBrand: item_?.brand ?? null,
        ingredientId: ingredient?.ingredientID ?? null,
        ingredientName: ingredient?.name ?? null,
        quantity: null,
        unitId: null,
        unitName: null,
        section: null,
        displayOrder: null,
        notes: null,
        isOptional: null,
        orphaned: false,
      });
      return;
    }
    onApply({
      ...anchor,
      kind: "adjust",
      itemId: null,
      itemName: null,
      itemBrand: null,
      ingredientId: null,
      ingredientName: null,
      quantity: qty === "" ? null : Number(qty),
      unitId,
      unitName,
      section: null,
      displayOrder: null,
      notes: notes === "" ? null : notes,
      isOptional: optional,
      orphaned: false,
    });
  };

  return (
    <Box
      sx={{
        display: "flex",
        gap: 2,
        flexWrap: "wrap",
        alignItems: "center",
        py: 1,
        px: 1,
        bgcolor: "action.hover",
        borderRadius: 1,
      }}
    >
      <RadioGroup
        row
        value={mode}
        onChange={(e) => setMode(e.target.value as typeof mode)}
      >
        <FormControlLabel value="substitute" control={<Radio size="small" />} label="Swap" />
        <FormControlLabel value="adjust" control={<Radio size="small" />} label="Adjust" />
        <FormControlLabel value="remove" control={<Radio size="small" />} label="Remove" />
      </RadioGroup>
      {mode === "substitute" && (
        <>
          <IngredientAutocomplete
            value={ingredient}
            onChange={setIngredient}
            onSelect={(ing) => void api.recordSelection("ingredient", ing.ingredientID)}
            sx={{ minWidth: 220 }}
          />
          <ItemPicker
            value={item_}
            onChange={(it) => {
              setItem(it);
              if (it && !ingredient) {
                const linked = it.householdIngredient ?? it.ingredient;
                if (linked) setIngredient(linked);
              }
            }}
          />
        </>
      )}
      {mode === "adjust" && (
        <>
          <TextField
            size="small"
            label="Portion"
            type="number"
            value={qty}
            slotProps={{ htmlInput: { min: 0, step: "any" } }}
            onChange={(e) => setQty(e.target.value)}
            sx={{ width: 110 }}
          />
          <UnitSelect value={unitId} onChange={setUnitId} units={units} />
          <TextField
            size="small"
            label="Notes"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            sx={{ minWidth: 160 }}
          />
          <FormControlLabel
            control={
              <Checkbox checked={optional} onChange={(e) => setOptional(e.target.checked)} />
            }
            label="Optional"
          />
        </>
      )}
      {mode === "remove" && (
        <Typography variant="body2" color="text.secondary">
          This line won't appear in your household's version of the recipe.
        </Typography>
      )}
      <Button
        size="small"
        variant="contained"
        onClick={apply}
        disabled={
          (mode === "substitute" && !ingredient && !item_) ||
          (mode === "adjust" && qty !== "" && Number(qty) <= 0)
        }
      >
        Apply tweak
      </Button>
      <Button size="small" onClick={onCancel}>
        Cancel
      </Button>
    </Box>
  );
}

/**
 * Inline editor for one step — replace rewrites instruction/timing,
 * remove drops the step from the household version.
 */
export function StepTweakEditor({
  step,
  existing,
  onApply,
  onCancel,
}: {
  step: RecipeStep;
  existing: StepDraft | null;
  onApply: (row: StepDraft) => void;
  onCancel: () => void;
}) {
  const [mode, setMode] = useState<"replace" | "remove">(
    existing?.kind === "add" ? "replace" : (existing?.kind ?? "replace")
  );
  const [instruction, setInstruction] = useState(
    existing?.instruction ?? step.instruction
  );
  const [duration, setDuration] = useState(
    numOrBlank(existing?.durationMinutes ?? step.durationMinutes)
  );
  const [stepType, setStepType] = useState(existing?.stepType ?? step.stepType ?? "");
  const [passive, setPassive] = useState(existing?.isPassive ?? step.isPassive ?? false);
  const [appliance, setAppliance] = useState(existing?.appliance ?? step.appliance ?? "");

  const apply = () => {
    if (!step.recipeStepID) return;
    const anchor = { key: `step-${step.recipeStepID}`, stepId: step.recipeStepID };
    if (mode === "remove") {
      onApply({
        ...anchor,
        kind: "remove",
        stepNumber: null,
        instruction: null,
        durationMinutes: null,
        stepType: null,
        isPassive: null,
        dependsOnStepNumber: null,
        appliance: null,
        orphaned: false,
      });
      return;
    }
    onApply({
      ...anchor,
      kind: "replace",
      stepNumber: step.stepNumber,
      instruction,
      durationMinutes: duration === "" ? null : Number(duration),
      stepType: stepType === "" ? null : stepType,
      isPassive: passive,
      dependsOnStepNumber: step.dependsOnStepNumber ?? null,
      appliance: appliance === "" ? null : appliance,
      orphaned: false,
    });
  };

  return (
    <Box
      sx={{
        display: "flex",
        gap: 2,
        flexWrap: "wrap",
        alignItems: "center",
        py: 1,
        px: 1,
        bgcolor: "action.hover",
        borderRadius: 1,
      }}
    >
      <RadioGroup
        row
        value={mode}
        onChange={(e) => setMode(e.target.value as typeof mode)}
      >
        <FormControlLabel value="replace" control={<Radio size="small" />} label="Edit" />
        <FormControlLabel value="remove" control={<Radio size="small" />} label="Remove" />
      </RadioGroup>
      {mode === "replace" && (
        <>
          <TextField
            size="small"
            label="Replacement directions"
            value={instruction}
            onChange={(e) => setInstruction(e.target.value)}
            sx={{ flexGrow: 1, minWidth: 220 }}
          />
          <StepMetaFields
            duration={duration}
            setDuration={setDuration}
            stepType={stepType}
            setStepType={setStepType}
            passive={passive}
            setPassive={setPassive}
            appliance={appliance}
            setAppliance={setAppliance}
          />
        </>
      )}
      {mode === "remove" && (
        <Typography variant="body2" color="text.secondary">
          This step won't appear in your household's version of the recipe.
        </Typography>
      )}
      <Button
        size="small"
        variant="contained"
        onClick={apply}
        disabled={mode === "replace" && instruction.trim() === ""}
      >
        Apply tweak
      </Button>
      <Button size="small" onClick={onCancel}>
        Cancel
      </Button>
    </Box>
  );
}

/**
 * The household tweaks panel — lists the working change set, adds lines
 * and steps, and writes the whole draft through setRecipeDelta.
 */
export function RecipeDeltaPanel({
  recipeId,
  delta,
  draft,
  onDraftChange,
  canonicalItems,
  canonicalSteps,
}: {
  recipeId: number;
  delta: RecipeDelta | null;
  draft: DeltaDraft;
  onDraftChange: (d: DeltaDraft) => void;
  canonicalItems: RecipeItem[];
  canonicalSteps: RecipeStep[];
}) {
  const queryClient = useQueryClient();
  const unitsQuery = useQuery({
    queryKey: ["units"],
    queryFn: api.getUnits,
    staleTime: 60_000,
  });
  const units = unitsQuery.data ?? [];

  const eventsQuery = useQuery({
    queryKey: ["recipeDeltaEvents", recipeId],
    queryFn: () => api.recipeDeltaEvents(recipeId, 10),
    staleTime: 30_000,
  });
  const events = eventsQuery.data ?? [];

  // add-line form
  const [addIngredient, setAddIngredient] = useState<Ingredient | null>(null);
  const [addItem, setAddItem] = useState<Item | null>(null);
  const [addQty, setAddQty] = useState("");
  const [addUnitId, setAddUnitId] = useState<number | null>(null);
  const [addOptional, setAddOptional] = useState(false);
  // add-step form
  const [addStepNumber, setAddStepNumber] = useState("");
  const [addInstruction, setAddInstruction] = useState("");
  const [addDuration, setAddDuration] = useState("");
  const [addStepType, setAddStepType] = useState("");
  const [addPassive, setAddPassive] = useState(false);
  const [addAppliance, setAddAppliance] = useState("");

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ["recipe", recipeId] });
    void queryClient.invalidateQueries({ queryKey: ["recipe-items", recipeId] });
    void queryClient.invalidateQueries({ queryKey: ["recipe-steps", recipeId] });
    void queryClient.invalidateQueries({ queryKey: ["recipeDeltaEvents", recipeId] });
  };

  const saveMutation = useMutation({
    mutationFn: () =>
      api.setRecipeDelta(
        recipeId,
        itemDraftInputs(draft.items),
        stepDraftInputs(draft.steps)
      ),
    onSuccess: invalidate,
  });

  const clearMutation = useMutation({
    mutationFn: () => api.clearRecipeDelta(recipeId),
    onSuccess: invalidate,
  });

  const dirty = draftIsDirty(draft, delta);
  const itemBase = new Map(
    canonicalItems.map((i) => [i.recipeItemID ?? 0, i])
  );
  const stepBase = new Map(
    canonicalSteps.map((s) => [s.recipeStepID, s])
  );
  const unitName = (id: number | null) =>
    id != null ? units.find((u) => u.unitID === id)?.name ?? null : null;

  const addLine = () => {
    if ((!addIngredient && !addItem) || addQty === "" || Number(addQty) <= 0)
      return;
    onDraftChange({
      ...draft,
      items: [
        ...draft.items,
        {
          key: nextKey(),
          recipeItemId: null,
          kind: "add",
          itemId: addItem?.itemID ?? null,
          itemName: addItem?.name ?? null,
          itemBrand: addItem?.brand ?? null,
          ingredientId: addIngredient?.ingredientID ?? null,
          ingredientName: addIngredient?.name ?? null,
          quantity: Number(addQty),
          unitId: addUnitId,
          unitName: unitName(addUnitId),
          section: null,
          displayOrder: null,
          notes: null,
          isOptional: addOptional,
          orphaned: false,
        },
      ],
    });
    setAddIngredient(null);
    setAddItem(null);
    setAddQty("");
    setAddUnitId(null);
    setAddOptional(false);
  };

  const addStep = () => {
    if (addStepNumber === "" || addInstruction.trim() === "") return;
    onDraftChange({
      ...draft,
      steps: [
        ...draft.steps,
        {
          key: nextKey(),
          stepId: null,
          kind: "add",
          stepNumber: Number(addStepNumber),
          instruction: addInstruction,
          durationMinutes: addDuration === "" ? null : Number(addDuration),
          stepType: addStepType === "" ? null : addStepType,
          isPassive: addPassive,
          dependsOnStepNumber: null,
          appliance: addAppliance === "" ? null : addAppliance,
          orphaned: false,
        },
      ],
    });
    setAddStepNumber("");
    setAddInstruction("");
    setAddDuration("");
    setAddStepType("");
    setAddPassive(false);
    setAddAppliance("");
  };

  return (
    <Paper sx={{ p: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <TuneIcon color="primary" fontSize="small" />
        <Typography variant="h5">Household tweaks</Typography>
      </Box>
      <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
        Tweaks apply for everyone in your household — the original recipe
        stays unchanged. Use <b>Tweak</b> on any line or step, or add your
        own below.
      </Typography>

      {saveMutation.error && (
        <Alert severity="error" sx={{ mt: 1 }}>
          {(saveMutation.error as Error).message}
        </Alert>
      )}
      {clearMutation.error && (
        <Alert severity="error" sx={{ mt: 1 }}>
          {(clearMutation.error as Error).message}
        </Alert>
      )}

      <Box sx={{ mt: 1 }}>
        {draft.items.length === 0 && draft.steps.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            No tweaks yet — the household sees the recipe as written.
          </Typography>
        )}
        {draft.items.map((d) => (
          <Box
            key={d.key}
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              py: 0.5,
              flexWrap: "wrap",
            }}
          >
            <Chip
              size="small"
              variant="outlined"
              label={itemKindChip[d.kind]}
              sx={{ minWidth: 64 }}
            />
            <Typography variant="body2" sx={{ flex: "1 1 200px", minWidth: 0 }}>
              {describeItemChange(
                d,
                d.recipeItemId != null ? itemBase.get(d.recipeItemId) : null
              )}
            </Typography>
            {d.orphaned && (
              <Chip
                size="small"
                color="warning"
                label="No longer applies"
              />
            )}
            <IconButton
              size="small"
              aria-label="Remove tweak"
              onClick={() =>
                onDraftChange({
                  ...draft,
                  items: removeDraftRow(draft.items, d.key),
                })
              }
            >
              <CloseIcon fontSize="small" />
            </IconButton>
          </Box>
        ))}
        {draft.steps.map((d) => (
          <Box
            key={d.key}
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              py: 0.5,
              flexWrap: "wrap",
            }}
          >
            <Chip
              size="small"
              variant="outlined"
              label={stepKindChip[d.kind]}
              sx={{ minWidth: 64 }}
            />
            <Typography variant="body2" sx={{ flex: "1 1 200px", minWidth: 0 }}>
              {describeStepChange(
                d,
                d.stepId != null ? stepBase.get(d.stepId) : null
              )}
            </Typography>
            {d.orphaned && (
              <Chip size="small" color="warning" label="No longer applies" />
            )}
            <IconButton
              size="small"
              aria-label="Remove tweak"
              onClick={() =>
                onDraftChange({
                  ...draft,
                  steps: removeDraftRow(draft.steps, d.key),
                })
              }
            >
              <CloseIcon fontSize="small" />
            </IconButton>
          </Box>
        ))}
      </Box>

      <Divider sx={{ my: 2 }} />
      <Typography variant="subtitle2" color="text.secondary" gutterBottom>
        Add an ingredient
      </Typography>
      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", alignItems: "center" }}>
        <IngredientAutocomplete
          value={addIngredient}
          onChange={setAddIngredient}
          sx={{ minWidth: 220 }}
        />
        <ItemPicker
          value={addItem}
          onChange={(it) => {
            setAddItem(it);
            if (it && !addIngredient) {
              const linked = it.householdIngredient ?? it.ingredient;
              if (linked) setAddIngredient(linked);
            }
          }}
        />
        <TextField
          size="small"
          label="Portion"
          type="number"
          value={addQty}
          slotProps={{ htmlInput: { min: 0, step: "any" } }}
          onChange={(e) => setAddQty(e.target.value)}
          sx={{ width: 110 }}
        />
        <UnitSelect value={addUnitId} onChange={setAddUnitId} units={units} />
        <FormControlLabel
          control={
            <Checkbox
              checked={addOptional}
              onChange={(e) => setAddOptional(e.target.checked)}
            />
          }
          label="Optional"
        />
        <Button
          size="small"
          variant="outlined"
          onClick={addLine}
          disabled={
            (!addIngredient && !addItem) || addQty === "" || Number(addQty) <= 0
          }
        >
          Add line
        </Button>
      </Box>

      <Typography variant="subtitle2" color="text.secondary" sx={{ mt: 2 }} gutterBottom>
        Add a step
      </Typography>
      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", alignItems: "center" }}>
        <TextField
          size="small"
          label="At step"
          type="number"
          value={addStepNumber}
          onChange={(e) => setAddStepNumber(e.target.value)}
          sx={{ width: 90 }}
        />
        <TextField
          size="small"
          label="Step directions"
          value={addInstruction}
          onChange={(e) => setAddInstruction(e.target.value)}
          sx={{ flexGrow: 1, minWidth: 220 }}
        />
        <StepMetaFields
          duration={addDuration}
          setDuration={setAddDuration}
          stepType={addStepType}
          setStepType={setAddStepType}
          passive={addPassive}
          setPassive={setAddPassive}
          appliance={addAppliance}
          setAppliance={setAddAppliance}
        />
        <Button
          size="small"
          variant="outlined"
          onClick={addStep}
          disabled={addStepNumber === "" || addInstruction.trim() === ""}
        >
          Add new step
        </Button>
      </Box>

      <Divider sx={{ my: 2 }} />
      <Box sx={{ display: "flex", gap: 1.5, alignItems: "center", flexWrap: "wrap" }}>
        <Button
          variant="contained"
          size="small"
          onClick={() => saveMutation.mutate()}
          disabled={!dirty || saveMutation.isPending}
        >
          {saveMutation.isPending ? "Saving…" : "Save tweaks"}
        </Button>
        <Button
          size="small"
          onClick={() => onDraftChange(draftFromDelta(delta))}
          disabled={!dirty}
        >
          Discard changes
        </Button>
        {delta && (
          <Button
            size="small"
            color="error"
            onClick={() => {
              if (window.confirm("Remove every household tweak from this recipe?")) {
                clearMutation.mutate();
              }
            }}
            disabled={clearMutation.isPending}
          >
            Clear all tweaks
          </Button>
        )}
        {dirty && (
          <Typography variant="caption" color="warning.dark">
            Unsaved changes
          </Typography>
        )}
        {saveMutation.isSuccess && !dirty && (
          <Typography variant="caption" color="text.secondary">
            Saved — your household sees this version everywhere.
          </Typography>
        )}
      </Box>

      <Divider sx={{ my: 2 }} />
      <Typography variant="subtitle2" color="text.secondary" gutterBottom>
        Tweak history
      </Typography>
      {events.length === 0 && !eventsQuery.isPending && (
        <Typography variant="body2" color="text.secondary">
          No tweak history yet.
        </Typography>
      )}
      {events.map((e) => (
        <Box
          key={e.recipeDeltaEventID}
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            py: 0.25,
            flexWrap: "wrap",
          }}
        >
          <Chip
            size="small"
            variant="outlined"
            label={eventKindChip[e.event] ?? e.event}
            sx={{ minWidth: 64 }}
          />
          <Typography variant="body2" sx={{ flexGrow: 1, minWidth: 0 }}>
            {describeDeltaEvent(e)}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {new Date(e.createdAt).toLocaleString()}
          </Typography>
        </Box>
      ))}
    </Paper>
  );
}
