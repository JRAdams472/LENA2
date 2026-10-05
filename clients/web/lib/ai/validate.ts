// Client-side validators for locally generated structured suggestions —
// ports of the server filters in internal/ai/suggest*.go. Every validator
// runs against the same prepared contextJson the server assembles, so a
// model can't reference a recipe/cell/step that wasn't offered.

export interface RawMealSuggestion {
  recipeId: number;
  dayOfWeek: number;
  mealType: string;
  reason: string;
  usesExpiring?: string[];
}

export interface MealPlanContext {
  occupied?: string[];
  candidates?: { id: number }[];
  maxSuggestions?: number;
}

function cellKey(day: number, mealType: string): string {
  return `${day}/${mealType.toLowerCase().trim()}`;
}

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) : s;
}

// Port of filterSuggestions: unknown recipe IDs, occupied cells,
// out-of-range days, empty/overlong meal types, and duplicate cells or
// recipes are dropped; sorted by day then meal type; capped.
export function validateMealSuggestions(
  parsed: unknown,
  ctx: MealPlanContext
): RawMealSuggestion[] {
  const list = (parsed as { suggestions?: unknown })?.suggestions;
  if (!Array.isArray(list)) return [];
  const valid = new Set((ctx.candidates ?? []).map((c) => c.id));
  const occupied = new Set(ctx.occupied ?? []);
  const seen = new Set<string>();
  const seenRecipes = new Set<number>();
  const out: RawMealSuggestion[] = [];
  for (const raw of list) {
    const sg = raw as Partial<RawMealSuggestion>;
    if (
      typeof sg.recipeId !== "number" ||
      !valid.has(sg.recipeId) ||
      typeof sg.dayOfWeek !== "number" ||
      sg.dayOfWeek < 0 ||
      sg.dayOfWeek > 6
    ) {
      continue;
    }
    const mt = (sg.mealType ?? "").trim();
    if (mt === "" || mt.length > 40 || seenRecipes.has(sg.recipeId)) continue;
    const key = cellKey(sg.dayOfWeek, mt);
    if (occupied.has(key) || seen.has(key)) continue;
    seen.add(key);
    seenRecipes.add(sg.recipeId);
    out.push({
      recipeId: sg.recipeId,
      dayOfWeek: sg.dayOfWeek,
      mealType: mt,
      reason: truncate((sg.reason ?? "").trim(), 120),
      usesExpiring: Array.isArray(sg.usesExpiring)
        ? sg.usesExpiring.filter((x): x is string => typeof x === "string")
        : [],
    });
  }
  out.sort((a, b) =>
    a.dayOfWeek !== b.dayOfWeek
      ? a.dayOfWeek - b.dayOfWeek
      : a.mealType.localeCompare(b.mealType)
  );
  return out.slice(0, ctx.maxSuggestions ?? 6);
}

export interface RawPairing {
  bottleId: number | null;
  name: string;
  reason: string;
  inCellar: boolean;
}

export interface PairingsContext {
  cellar?: { bottleId: number; vineyard: string; vintageYear?: number }[];
}

// Port of filterPairings: bottleId must exist in the cellar (which then
// stamps name + inCellar), style picks need a real name, dedupe, cap.
export function validatePairings(
  parsed: unknown,
  ctx: PairingsContext,
  maxCount = 4
): RawPairing[] {
  const list = (parsed as { pairings?: unknown })?.pairings;
  if (!Array.isArray(list)) return [];
  const cellarById = new Map(
    (ctx.cellar ?? []).map((b) => [b.bottleId, b] as const)
  );
  const seen = new Set<string>();
  const out: RawPairing[] = [];
  for (const raw of list) {
    const p = toPairing(raw, cellarById);
    if (!p) continue;
    const key = `${p.bottleId ?? 0}|${p.name.toLowerCase()}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(p);
  }
  return out.slice(0, maxCount);
}

type CellarMap = Map<number, NonNullable<PairingsContext["cellar"]>[number]>;

// A numeric bottleId must resolve to a cellar bottle (stamps name +
// inCellar); a style pick needs a real bounded name.
function toPairing(raw: unknown, cellarById: CellarMap): RawPairing | null {
  const p = raw as { bottleId?: unknown; name?: unknown; reason?: unknown };
  let bottleId: number | null = null;
  let name = "";
  let inCellar = false;
  if (typeof p.bottleId === "number") {
    const b = cellarById.get(p.bottleId);
    if (!b) return null;
    bottleId = p.bottleId;
    name =
      b.vintageYear && b.vintageYear > 0
        ? `${b.vineyard} ${b.vintageYear}`
        : b.vineyard;
    inCellar = true;
  } else {
    if (typeof p.name !== "string") return null;
    name = p.name.trim();
    if (name === "" || name.length > 80) return null;
  }
  return {
    bottleId,
    name,
    reason: truncate(((p.reason as string) ?? "").trim(), 160),
    inCellar,
  };
}

export interface RawCocktail {
  recipeId: number;
  reason: string;
  missing: string[];
}

export interface CocktailsContext {
  cocktails?: { id: number }[];
}

// Port of filterCocktails: recipeId must be a cocktail candidate,
// missing-ingredient strings are bounded, inStockOnly drops anything with
// gaps, dedupe, cap.
export function validateCocktails(
  parsed: unknown,
  ctx: CocktailsContext,
  inStockOnly: boolean,
  maxCount = 6
): RawCocktail[] {
  const list = (parsed as { suggestions?: unknown })?.suggestions;
  if (!Array.isArray(list)) return [];
  const valid = new Set((ctx.cocktails ?? []).map((c) => c.id));
  const seen = new Set<number>();
  const out: RawCocktail[] = [];
  for (const raw of list) {
    const c = raw as {
      recipeId?: unknown;
      reason?: unknown;
      missingIngredients?: unknown;
    };
    if (typeof c.recipeId !== "number" || !valid.has(c.recipeId)) continue;
    if (seen.has(c.recipeId)) continue;
    const missing = (Array.isArray(c.missingIngredients) ? c.missingIngredients : [])
      .filter((m): m is string => typeof m === "string")
      .map((m) => m.trim())
      .filter((m) => m !== "" && m.length <= 80);
    if (inStockOnly && missing.length > 0) continue;
    seen.add(c.recipeId);
    out.push({
      recipeId: c.recipeId,
      reason: truncate(((c.reason as string) ?? "").trim(), 160),
      missing,
    });
  }
  return out.slice(0, maxCount);
}

export interface RawEventFix {
  eventRecipeId: number;
  recipeName: string;
  stepNumber: number | null;
  action: "shift_serve" | "set_appliance" | "set_duration" | "set_dependency";
  minutes: number | null;
  appliance: string | null;
  durationMinutes: number | null;
  dependsOnStepNumber: number | null;
  reason: string;
}

export interface EventContext {
  slotGranularityMinutes?: number;
  recipes?: {
    eventRecipeId: number;
    name: string;
    steps?: { step: number }[];
  }[];
}

// Port of filterEventFixes: eventRecipeId/stepNumber must exist in the
// timeline, each action's payload fields are range-checked, granularity
// enforced for shifts, dedupe, cap. RecipeName is stamped from context.
export function validateEventFixes(
  parsed: unknown,
  ctx: EventContext,
  maxCount = 6
): RawEventFix[] {
  const list = (parsed as { fixes?: unknown })?.fixes;
  if (!Array.isArray(list)) return [];
  const { stepsByRecipe, names } = eventFixIndex(ctx);
  const gran = ctx.slotGranularityMinutes && ctx.slotGranularityMinutes > 0
    ? ctx.slotGranularityMinutes
    : 15;
  const seen = new Set<string>();
  const out: RawEventFix[] = [];
  for (const raw of list) {
    const fix = toEventFix(raw, stepsByRecipe, names, gran);
    if (!fix) continue;
    const key = `${fix.eventRecipeId}|${fix.action}|${fix.stepNumber ?? 0}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(fix);
  }
  return out.slice(0, maxCount);
}

// One fix entry: known recipe + step references and a payload that passes
// the per-action checks. RecipeName is stamped from context.
function toEventFix(
  raw: unknown,
  stepsByRecipe: Map<number, Set<number>>,
  names: Map<number, string>,
  gran: number
): RawEventFix | null {
  const f = raw as Record<string, unknown>;
  const eventRecipeId = f.eventRecipeId;
  const action = f.action;
  if (typeof eventRecipeId !== "number" || typeof action !== "string") {
    return null;
  }
  const steps = stepsByRecipe.get(eventRecipeId);
  if (!steps) return null;
  const fields: EventFixFields = {
    stepNumber: typeof f.stepNumber === "number" ? f.stepNumber : null,
    minutes: typeof f.minutes === "number" ? f.minutes : null,
    appliance: typeof f.appliance === "string" ? f.appliance : null,
    durationMinutes:
      typeof f.durationMinutes === "number" ? f.durationMinutes : null,
    dependsOnStepNumber:
      typeof f.dependsOnStepNumber === "number" ? f.dependsOnStepNumber : null,
  };
  if (!eventFixPayloadOk(action, steps, gran, fields)) return null;
  return {
    eventRecipeId,
    recipeName: names.get(eventRecipeId) ?? "",
    action: action as RawEventFix["action"],
    ...fields,
    reason: truncate(((f.reason as string) ?? "").trim(), 160),
  };
}

interface EventFixFields {
  stepNumber: number | null;
  minutes: number | null;
  appliance: string | null;
  durationMinutes: number | null;
  dependsOnStepNumber: number | null;
}

// Steps-per-recipe and name lookups for the offered timeline.
function eventFixIndex(ctx: EventContext): {
  stepsByRecipe: Map<number, Set<number>>;
  names: Map<number, string>;
} {
  const stepsByRecipe = new Map<number, Set<number>>();
  const names = new Map<number, string>();
  for (const r of ctx.recipes ?? []) {
    names.set(r.eventRecipeId, r.name);
    stepsByRecipe.set(
      r.eventRecipeId,
      new Set((r.steps ?? []).map((s) => s.step))
    );
  }
  return { stepsByRecipe, names };
}

// Per-action payload validation: unknown actions and out-of-range fields are
// dropped; step references must point at real steps in the recipe.
function eventFixPayloadOk(
  action: string,
  steps: Set<number>,
  gran: number,
  f: EventFixFields
): boolean {
  const stepOk =
    f.stepNumber !== null && steps.has(f.stepNumber);
  switch (action) {
    case "shift_serve":
      return (
        f.minutes !== null &&
        f.minutes !== 0 &&
        f.minutes <= 480 &&
        f.minutes >= -480 &&
        f.minutes % gran === 0
      );
    case "set_appliance":
      return (
        stepOk &&
        f.appliance !== null &&
        f.appliance.trim() !== "" &&
        f.appliance.length <= 40
      );
    case "set_duration":
      return (
        stepOk &&
        f.durationMinutes !== null &&
        f.durationMinutes > 0 &&
        f.durationMinutes <= 720
      );
    case "set_dependency":
      return (
        stepOk &&
        f.dependsOnStepNumber !== null &&
        f.dependsOnStepNumber !== f.stepNumber &&
        steps.has(f.dependsOnStepNumber)
      );
    default:
      return false;
  }
}
