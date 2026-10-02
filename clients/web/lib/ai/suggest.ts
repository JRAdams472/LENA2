// Smart suggestion entry points — each tries the ready local engine first
// (server-prepared context, client-side validation), then falls back to
// the server's full inference path. A local engine is only used when it's
// already loaded or zero-download; nothing here ever starts a model
// download implicitly.

import { api, mealTypeToNumber } from "../api";
import type {
  CocktailSuggestion,
  EventFixSuggestion,
  MealPlanSuggestion,
  PairingSuggestion,
  Recipe,
} from "../types";
import { engineForSuggestions } from "./engineStore";
import { runStructured } from "./structured";
import {
  validateCocktails,
  validateEventFixes,
  validateMealSuggestions,
  validatePairings,
  type CocktailsContext,
  type PairingsContext,
} from "./validate";

// hydrateRecipes loads full Recipe objects for locally validated picks —
// the same rows the server resolver would load for Recipe fields.
async function hydrateRecipes(ids: number[]): Promise<Map<number, Recipe>> {
  const unique = [...new Set(ids)];
  const recipes = await Promise.all(unique.map((id) => api.getRecipe(id)));
  return new Map(unique.map((id, i) => [id, recipes[i]]));
}

export async function suggestMeals(
  mealPlanId: number,
  maxSuggestions = 6
): Promise<MealPlanSuggestion[]> {
  const engine = await engineForSuggestions();
  if (engine) {
    try {
      const prepared = await api.prepareAssistantRequest(
        "suggest-meals",
        JSON.stringify({ mealPlanId, maxSuggestions })
      );
      if (!prepared) return [];
      const rows = await runStructured(engine, prepared, validateMealSuggestions);
      const recipes = await hydrateRecipes(rows.map((r) => r.recipeId));
      return rows
        .filter((r) => recipes.has(r.recipeId))
        .map((r) => ({
          recipe: recipes.get(r.recipeId)!,
          dayOfWeek: r.dayOfWeek,
          mealType: mealTypeToNumber(r.mealType),
          reason: r.reason,
          usesExpiringItems: r.usesExpiring ?? [],
        }));
    } catch {
      /* fall through to the server path */
    }
  }
  return api.suggestMeals(mealPlanId, maxSuggestions);
}

export async function suggestEventFixes(
  foodEventId: number,
  maxSuggestions = 6
): Promise<EventFixSuggestion[]> {
  const engine = await engineForSuggestions();
  if (engine) {
    try {
      const prepared = await api.prepareAssistantRequest(
        "suggest-event-fixes",
        JSON.stringify({ foodEventId, maxSuggestions })
      );
      if (!prepared) return [];
      return await runStructured(engine, prepared, validateEventFixes);
    } catch {
      /* fall through */
    }
  }
  return api.suggestEventFixes(foodEventId, maxSuggestions);
}

export async function suggestPairings(
  recipeId: number,
  maxSuggestions = 4
): Promise<PairingSuggestion[]> {
  const engine = await engineForSuggestions();
  if (engine) {
    try {
      const prepared = await api.prepareAssistantRequest(
        "suggest-pairings",
        JSON.stringify({ recipeId, maxSuggestions })
      );
      if (!prepared) return [];
      return await runStructured(engine, prepared, (o, ctx) =>
        validatePairings(o, ctx as PairingsContext, maxSuggestions)
      );
    } catch {
      /* fall through */
    }
  }
  return api.suggestPairings(recipeId, maxSuggestions);
}

export async function suggestCocktails(
  inStockOnly = false,
  maxSuggestions = 6
): Promise<CocktailSuggestion[]> {
  const engine = await engineForSuggestions();
  if (engine) {
    try {
      const prepared = await api.prepareAssistantRequest(
        "suggest-cocktails",
        JSON.stringify({ maxSuggestions, inStockOnly })
      );
      if (!prepared) return [];
      const rows = await runStructured(engine, prepared, (o, ctx) =>
        validateCocktails(o, ctx as CocktailsContext, inStockOnly, maxSuggestions)
      );
      const recipes = await hydrateRecipes(rows.map((r) => r.recipeId));
      return rows
        .filter((r) => recipes.has(r.recipeId))
        .map((r) => ({
          recipe: recipes.get(r.recipeId)!,
          reason: r.reason,
          missingIngredients: r.missing,
        }));
    } catch {
      /* fall through */
    }
  }
  return api.suggestCocktails(inStockOnly, maxSuggestions);
}
