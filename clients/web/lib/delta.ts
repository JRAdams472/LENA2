import type { RecipeDeltaItemInput, RecipeDeltaStepInput } from "./api";
import { brandedName, fmtQty, recipeItemLabel } from "./format";
import type {
  RecipeDelta,
  RecipeDeltaItem,
  RecipeDeltaStep,
  RecipeItem,
  RecipeStep,
} from "./types";

// Draft rows are the editable mirror of a RecipeDelta change set. The UI
// mutates drafts, then serializes the whole set through setRecipeDelta —
// the server stores it verbatim (one change per anchored canonical line).

export type ItemDeltaKind = "substitute" | "adjust" | "remove" | "add";
export type StepDeltaKind = "replace" | "remove" | "add";

export interface ItemDraft {
  // Stable list key: "line-<recipeItemId>" for anchored rows,
  // "new-<n>" for additions created in the editor.
  key: string;
  recipeItemId: number | null;
  kind: ItemDeltaKind;
  itemId: number | null;
  itemName: string | null;
  itemBrand: string | null;
  ingredientId: number | null;
  ingredientName: string | null;
  quantity: number | null;
  unitId: number | null;
  unitName: string | null;
  section: string | null;
  displayOrder: number | null;
  notes: string | null;
  isOptional: boolean | null;
  orphaned: boolean;
}

export interface StepDraft {
  key: string;
  stepId: number | null;
  kind: StepDeltaKind;
  stepNumber: number | null;
  instruction: string | null;
  durationMinutes: number | null;
  stepType: string | null;
  isPassive: boolean | null;
  dependsOnStepNumber: number | null;
  appliance: string | null;
  orphaned: boolean;
}

export interface DeltaDraft {
  items: ItemDraft[];
  steps: StepDraft[];
}

export const emptyDraft: DeltaDraft = { items: [], steps: [] };

export const ITEM_DELTA_LABEL: Record<ItemDeltaKind, string> = {
  substitute: "Swapped",
  adjust: "Adjusted",
  remove: "Removed",
  add: "Added",
};

export const STEP_DELTA_LABEL: Record<StepDeltaKind, string> = {
  replace: "Edited",
  remove: "Removed",
  add: "Added",
};

/** Badge text for a deltaKind marker on an effective line or step. */
export function deltaBadgeLabel(kind: string | null | undefined): string {
  switch (kind) {
    case "substitute":
      return "Swapped";
    case "adjust":
      return "Adjusted";
    case "replace":
      return "Edited";
    case "add":
      return "Added";
    default:
      return "Tweaked";
  }
}

function itemDraftFromRow(i: RecipeDeltaItem): ItemDraft {
  return {
    key:
      i.recipeItemID != null
        ? `line-${i.recipeItemID}`
        : `saved-${i.recipeDeltaItemID}`,
    recipeItemId: i.recipeItemID,
    kind: i.kind,
    itemId: i.itemID,
    itemName: i.itemName,
    itemBrand: i.itemBrand,
    ingredientId: i.ingredientID,
    ingredientName: i.ingredientName,
    quantity: i.quantity,
    unitId: i.unitID,
    unitName: i.unitOfMeasure,
    section: i.section,
    displayOrder: i.displayOrder,
    notes: i.notes,
    isOptional: i.isOptional,
    orphaned: i.orphaned,
  };
}

function stepDraftFromRow(s: RecipeDeltaStep): StepDraft {
  return {
    key:
      s.stepID != null ? `step-${s.stepID}` : `saved-${s.recipeDeltaStepID}`,
    stepId: s.stepID,
    kind: s.kind,
    stepNumber: s.stepNumber,
    instruction: s.instruction,
    durationMinutes: s.durationMinutes,
    stepType: s.stepType,
    isPassive: s.isPassive,
    dependsOnStepNumber: s.dependsOnStepNumber,
    appliance: s.appliance,
    orphaned: s.orphaned,
  };
}

/** Clones the saved delta into an editable draft. */
export function draftFromDelta(delta: RecipeDelta | null | undefined): DeltaDraft {
  if (!delta) return { items: [], steps: [] };
  return {
    items: delta.items.map(itemDraftFromRow),
    steps: delta.steps.map(stepDraftFromRow),
  };
}

export function itemDraftInputs(items: ItemDraft[]): RecipeDeltaItemInput[] {
  return items.map((d) => ({
    recipeItemId: d.recipeItemId != null ? String(d.recipeItemId) : null,
    kind: d.kind,
    itemId: d.itemId != null ? String(d.itemId) : null,
    ingredientId: d.ingredientId != null ? String(d.ingredientId) : null,
    quantity: d.quantity,
    unitId: d.unitId != null ? String(d.unitId) : null,
    section: d.section,
    displayOrder: d.displayOrder,
    notes: d.notes,
    isOptional: d.isOptional,
  }));
}

export function stepDraftInputs(steps: StepDraft[]): RecipeDeltaStepInput[] {
  return steps.map((d) => ({
    stepId: d.stepId != null ? String(d.stepId) : null,
    kind: d.kind,
    stepNumber: d.stepNumber,
    instruction: d.instruction,
    durationMinutes: d.durationMinutes,
    stepType: d.stepType,
    isPassive: d.isPassive,
    dependsOnStepNumber: d.dependsOnStepNumber,
    appliance: d.appliance,
  }));
}

// Normalized shape for equality — keys fixed, undefined collapsed to null,
// key/orphaned/display names ignored (they're local bookkeeping).
function normItem(d: ItemDraft) {
  return {
    recipeItemId: d.recipeItemId,
    kind: d.kind,
    itemId: d.itemId,
    ingredientId: d.ingredientId,
    quantity: d.quantity,
    unitId: d.unitId,
    section: d.section ?? null,
    displayOrder: d.displayOrder ?? null,
    notes: d.notes ?? null,
    isOptional: d.isOptional ?? null,
  };
}

function normStep(d: StepDraft) {
  return {
    stepId: d.stepId,
    kind: d.kind,
    stepNumber: d.stepNumber,
    instruction: d.instruction,
    durationMinutes: d.durationMinutes,
    stepType: d.stepType ?? null,
    isPassive: d.isPassive ?? null,
    dependsOnStepNumber: d.dependsOnStepNumber,
    appliance: d.appliance ?? null,
  };
}

const byAnchor = (a: { recipeItemId?: number | null; stepId?: number | null; key?: string; kind: string }, b: typeof a) =>
  (a.recipeItemId ?? a.stepId ?? 0) - (b.recipeItemId ?? b.stepId ?? 0) ||
  a.kind.localeCompare(b.kind);

/** True when the draft no longer matches the saved change set. */
export function draftIsDirty(draft: DeltaDraft, delta: RecipeDelta | null | undefined): boolean {
  const saved = draftFromDelta(delta);
  const sortItems = (xs: ItemDraft[]) => [...xs].sort(byAnchor);
  const sortSteps = (xs: StepDraft[]) => [...xs].sort(byAnchor);
  return (
    JSON.stringify(sortItems(draft.items).map(normItem)) !==
      JSON.stringify(sortItems(saved.items).map(normItem)) ||
    JSON.stringify(sortSteps(draft.steps).map(normStep)) !==
      JSON.stringify(sortSteps(saved.steps).map(normStep))
  );
}

/**
 * Upserts a change for one canonical line. Anchored rows share one slot
 * per recipeItemId (the backend enforces it), so a new tweak on a tweaked
 * line replaces the existing row.
 */
export function upsertItemDraft(items: ItemDraft[], row: ItemDraft): ItemDraft[] {
  if (row.kind === "add" || row.recipeItemId == null) {
    return [...items, row];
  }
  const rest = items.filter(
    (d) => d.recipeItemId !== row.recipeItemId || d.kind === "add"
  );
  return [...rest, row];
}

export function upsertStepDraft(steps: StepDraft[], row: StepDraft): StepDraft[] {
  if (row.kind === "add" || row.stepId == null) {
    return [...steps, row];
  }
  const rest = steps.filter((d) => d.stepId !== row.stepId || d.kind === "add");
  return [...rest, row];
}

export function removeDraftRow<T extends { key: string }>(rows: T[], key: string): T[] {
  return rows.filter((d) => d.key !== key);
}

/** The draft row anchoring a canonical line, if any. */
export function draftForLine(items: ItemDraft[], recipeItemId: number): ItemDraft | null {
  return (
    items.find((d) => d.recipeItemId === recipeItemId && d.kind !== "add") ??
    null
  );
}

export function draftForStep(steps: StepDraft[], stepId: number): StepDraft | null {
  return (
    steps.find((d) => d.stepId === stepId && d.kind !== "add") ?? null
  );
}

/** Display name of the ingredient/brand a draft row points at. */
export function draftTargetLabel(d: {
  ingredientName?: string | null;
  itemName?: string | null;
  itemBrand?: string | null;
}): string {
  const bits: string[] = [];
  if (d.ingredientName) bits.push(d.ingredientName);
  if (d.itemName) bits.push(brandedName(d.itemBrand, d.itemName));
  return bits.join(" — ") || "ingredient";
}

/** Human-readable summary of one item change for the tweaks list. */
export function describeItemChange(
  d: ItemDraft,
  base?: RecipeItem | null
): string {
  const resolved = base ? recipeItemLabel(base) : "";
  const baseLabel =
    resolved ||
    (d.recipeItemId != null ? `line ${d.recipeItemId}` : "the original line");
  const target = draftTargetLabel(d);
  switch (d.kind) {
    case "substitute":
      return `Swap ${baseLabel} for ${target}`;
    case "adjust": {
      const parts: string[] = [];
      if (d.quantity != null)
        parts.push(`${fmtQty(d.quantity)}${d.unitName ? ` ${d.unitName}` : ""}`.trim());
      if (d.notes) parts.push(`"${d.notes}"`);
      if (d.isOptional != null)
        parts.push(d.isOptional ? "optional" : "required");
      return `Adjust ${baseLabel}${parts.length ? ` — ${parts.join(", ")}` : ""}`;
    }
    case "remove":
      return `Remove ${baseLabel}`;
    case "add": {
      const qty =
        d.quantity != null
          ? `${fmtQty(d.quantity)}${d.unitName ? ` ${d.unitName}` : ""} `
          : "";
      return `Add ${qty}${target}`.replace(/\s+/g, " ").trim();
    }
  }
}

/** Human-readable summary of one step change for the tweaks list. */
export function describeStepChange(
  d: StepDraft,
  base?: RecipeStep | null
): string {
  const n = base?.stepNumber ?? d.stepNumber;
  const baseLabel = n != null ? `step ${n}` : "the original step";
  switch (d.kind) {
    case "replace":
      return `Edit ${baseLabel}${d.instruction ? ` — "${d.instruction}"` : ""}`;
    case "remove":
      return `Remove ${baseLabel}`;
    case "add":
      return `Add step ${d.stepNumber ?? "?"}${d.instruction ? ` — "${d.instruction}"` : ""}`;
  }
}
