import {
  validateCocktails,
  validateEventFixes,
  validateMealSuggestions,
  validatePairings,
} from "@/lib/ai/validate";

describe("validateMealSuggestions", () => {
  const ctx = {
    occupied: ["1/dinner"],
    candidates: [{ id: 1 }, { id: 2 }, { id: 3 }],
    maxSuggestions: 6,
  };
  const s = (over: object) => ({
    recipeId: 1,
    dayOfWeek: 2,
    mealType: "Dinner",
    reason: "fits",
    ...over,
  });

  it("keeps valid picks and stamps bounded fields", () => {
    const out = validateMealSuggestions({ suggestions: [s({})] }, ctx);
    expect(out).toHaveLength(1);
    expect(out[0].mealType).toBe("Dinner");
  });

  it("drops unknown recipe ids, occupied cells, and bad days", () => {
    const out = validateMealSuggestions(
      {
        suggestions: [
          s({ recipeId: 99 }),
          s({ dayOfWeek: 1 }),
          s({ dayOfWeek: 9 }),
          s({ recipeId: 2 }),
        ],
      },
      ctx
    );
    expect(out).toHaveLength(1);
    expect(out[0].recipeId).toBe(2);
  });

  it("dedupes recipes and cells and caps at maxSuggestions", () => {
    const out = validateMealSuggestions(
      {
        suggestions: [
          s({ recipeId: 1 }),
          s({ recipeId: 1, dayOfWeek: 3 }),
          s({ recipeId: 2 }),
        ],
      },
      { ...ctx, maxSuggestions: 1 }
    );
    expect(out).toHaveLength(1);
    expect(out[0].recipeId).toBe(1);
  });

  it("returns [] when the output has no suggestions array", () => {
    expect(validateMealSuggestions({}, ctx)).toEqual([]);
  });
});

describe("validatePairings", () => {
  const ctx = {
    cellar: [
      { bottleId: 7, vineyard: "Willamette Vineyards", vintageYear: 2019 },
      { bottleId: 8, vineyard: "Bare Bones", vintageYear: 0 },
    ],
  };

  it("stamps cellar name and inCellar for bottle picks", () => {
    const out = validatePairings(
      { pairings: [{ bottleId: 7, name: "ignored", reason: "acid match" }] },
      ctx
    );
    expect(out[0]).toEqual({
      bottleId: 7,
      name: "Willamette Vineyards 2019",
      reason: "acid match",
      inCellar: true,
    });
  });

  it("drops unknown bottleIds and empty style names", () => {
    const out = validatePairings(
      {
        pairings: [
          { bottleId: 99, name: "x", reason: "r" },
          { name: "  ", reason: "r" },
          { name: "Pinot Noir", reason: "classic" },
        ],
      },
      ctx
    );
    expect(out).toHaveLength(1);
    expect(out[0]).toMatchObject({ bottleId: null, name: "Pinot Noir", inCellar: false });
  });
});

describe("validateCocktails", () => {
  const ctx = { cocktails: [{ id: 11 }, { id: 12 }] };

  it("validates recipe ids against the cocktail candidates", () => {
    const out = validateCocktails(
      { suggestions: [{ recipeId: 11, reason: "r" }, { recipeId: 99, reason: "r" }] },
      ctx,
      false
    );
    expect(out).toHaveLength(1);
    expect(out[0].recipeId).toBe(11);
  });

  it("drops picks with missing ingredients when inStockOnly", () => {
    const out = validateCocktails(
      {
        suggestions: [
          { recipeId: 11, reason: "r", missingIngredients: ["bitters"] },
          { recipeId: 12, reason: "r" },
        ],
      },
      ctx,
      true
    );
    expect(out).toHaveLength(1);
    expect(out[0].recipeId).toBe(12);
  });
});

describe("validateEventFixes", () => {
  const ctx = {
    slotGranularityMinutes: 15,
    recipes: [
      {
        eventRecipeId: 3,
        name: "Turkey",
        steps: [{ step: 1 }, { step: 2 }],
      },
    ],
  };

  it("accepts a valid shift_serve and stamps the recipe name", () => {
    const out = validateEventFixes(
      { fixes: [{ eventRecipeId: 3, action: "shift_serve", minutes: 30, reason: "frees oven" }] },
      ctx
    );
    expect(out[0]).toMatchObject({ recipeName: "Turkey", minutes: 30 });
  });

  it("rejects non-granularity shifts, unknown steps, and bad actions", () => {
    const out = validateEventFixes(
      {
        fixes: [
          { eventRecipeId: 3, action: "shift_serve", minutes: 17, reason: "x" },
          { eventRecipeId: 3, action: "set_appliance", stepNumber: 9, appliance: "oven", reason: "x" },
          { eventRecipeId: 3, action: "delete_dish", reason: "x" },
          { eventRecipeId: 3, action: "set_duration", stepNumber: 1, durationMinutes: 45, reason: "x" },
        ],
      },
      ctx
    );
    expect(out).toHaveLength(1);
    expect(out[0].action).toBe("set_duration");
  });

  it("rejects self-dependency and unknown dependency targets", () => {
    const out = validateEventFixes(
      {
        fixes: [
          { eventRecipeId: 3, action: "set_dependency", stepNumber: 1, dependsOnStepNumber: 1, reason: "x" },
          { eventRecipeId: 3, action: "set_dependency", stepNumber: 1, dependsOnStepNumber: 2, reason: "ok" },
        ],
      },
      ctx
    );
    expect(out).toHaveLength(1);
  });
});
