import { api, ApiError } from "@/lib/api";

const mockFetch = global.fetch as jest.Mock;

function mockGraphQL(data: object, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: {
      get: (name: string) =>
        name === "content-type" ? "application/json" : null,
    },
    json: async () => ({ data }),
  };
}

function lastBody(): {
  query: string;
  variables: { input: Record<string, unknown> } & Record<string, unknown>;
} {
  const call = mockFetch.mock.calls.at(-1);
  return JSON.parse((call?.[1] as RequestInit).body as string);
}

const gqlEvent = {
  id: "3",
  name: "Friendsgiving",
  eventDate: "2026-11-26",
  slotGranularityMinutes: 30,
  isActive: true,
  recipes: [],
};

describe("food event api client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("getFoodEventsPaged maps the page", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        foodEvents: {
          items: [gqlEvent],
          pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
        },
      })
    );

    const result = await api.getFoodEventsPaged(1, 25);

    expect(result.items).toHaveLength(1);
    expect(result.items[0].foodEventID).toBe(3);
    expect(result.items[0].name).toBe("Friendsgiving");
    expect(result.items[0].slotGranularityMinutes).toBe(30);
    expect(result.items[0].eventRecipes).toEqual([]);
    expect(result.totalCount).toBe(1);
  });

  it("getFoodEvent maps nested recipes", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        foodEvent: {
          ...gqlEvent,
          recipes: [
            {
              id: "9",
              mealType: "dinner",
              targetTime: "2026-11-26T18:30:00Z",
              servings: 8,
              notes: "turkey",
              recipe: {
                id: "11",
                name: "Turkey",
                description: null,
                servings: 10,
                prepTimeMinutes: null,
                cookTimeMinutes: null,
                items: [],
                steps: [],
                isFavorite: false,
                selectionCount: 0,
                personalSelectionCount: 0,
                myRating: null,
                averageRating: null,
                ratingCount: 0,
              },
            },
          ],
        },
      })
    );

    const ev = await api.getFoodEvent(3);

    expect(ev.eventRecipes).toHaveLength(1);
    const slot = ev.eventRecipes![0];
    expect(slot.eventRecipeID).toBe(9);
    expect(slot.foodEventID).toBe(3);
    expect(slot.recipeID).toBe(11);
    expect(slot.recipe?.recipeName).toBe("Turkey");
    expect(slot.servings).toBe(8);
    expect(slot.notes).toBe("turkey");
  });

  it("getFoodEvent throws 404 when the event is null", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ foodEvent: null }));
    const error = await api.getFoodEvent(99).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(404);
  });

  it("createFoodEvent sends the create input", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ createFoodEvent: gqlEvent })
    );

    const ev = await api.createFoodEvent({
      name: "Friendsgiving",
      eventDate: "2026-11-26",
      slotGranularityMinutes: 30,
    });

    const body = lastBody();
    expect(body.query).toContain("createFoodEvent");
    expect(body.variables.input).toEqual({
      name: "Friendsgiving",
      eventDate: "2026-11-26",
      slotGranularityMinutes: 30,
    });
    expect(ev.foodEventID).toBe(3);
  });

  it("updateFoodEvent sends partial fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ updateFoodEvent: { ...gqlEvent, name: "Renamed" } })
    );

    const ev = await api.updateFoodEvent(3, { name: "Renamed" });

    const body = lastBody();
    expect(body.variables.id).toBe("3");
    expect(body.variables.input).toEqual({
      name: "Renamed",
      eventDate: null,
      slotGranularityMinutes: null,
      isActive: null,
    });
    expect(ev.name).toBe("Renamed");
  });

  it("deleteFoodEvent posts the mutation", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ deleteFoodEvent: true })
    );

    await api.deleteFoodEvent(3);

    const body = lastBody();
    expect(body.query).toContain("deleteFoodEvent");
    expect(body.variables.id).toBe("3");
  });

  it("addEventRecipe sends the slot input", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        addEventRecipe: {
          id: "9",
          mealType: "dinner",
          targetTime: "2026-11-26T18:30:00Z",
          servings: 8,
          notes: null,
          recipe: null,
        },
      })
    );

    const slot = await api.addEventRecipe(3, {
      recipeID: 11,
      mealType: "dinner",
      targetTime: "2026-11-26T18:30:00Z",
      servings: 8,
    });

    const body = lastBody();
    expect(body.variables.input).toEqual({
      foodEventId: "3",
      recipeId: "11",
      mealType: "dinner",
      targetTime: "2026-11-26T18:30:00Z",
      servings: 8,
      notes: null,
    });
    expect(slot.eventRecipeID).toBe(9);
    expect(slot.foodEventID).toBe(3);
    expect(slot.recipeID).toBeNull();
  });

  it("updateEventRecipe sends partial slot fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateEventRecipe: {
          id: "9",
          mealType: "dinner",
          targetTime: "2026-11-26T19:00:00Z",
          servings: 10,
          notes: null,
          recipe: null,
        },
      })
    );

    const slot = await api.updateEventRecipe(9, { servings: 10 });

    const body = lastBody();
    expect(body.variables.id).toBe("9");
    expect(body.variables.input.servings).toBe(10);
    expect(slot.servings).toBe(10);
  });

  it("removeEventRecipe posts the mutation", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ removeEventRecipe: true })
    );

    await api.removeEventRecipe(9);

    const body = lastBody();
    expect(body.query).toContain("removeEventRecipe");
    expect(body.variables.id).toBe("9");
  });

  it("getEventTimeline maps recipes and steps", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        eventTimeline: {
          foodEventId: "3",
          warnings: ["appliance conflict: oven"],
          recipes: [
            {
              eventRecipeId: "9",
              name: "Casserole",
              targetTime: "2026-11-26T18:30:00Z",
              servings: 8,
              startBy: "2026-11-26T17:30:00Z",
              unschedulable: false,
              warnings: ["1 step(s) have no duration — estimated"],
              steps: [
                {
                  stepNumber: 1,
                  instruction: "mix",
                  stepType: "prep",
                  isPassive: false,
                  appliance: null,
                  durationMinutes: null,
                  scheduledMinutes: 30,
                  estimated: true,
                  startTime: "2026-11-26T17:30:00Z",
                  endTime: "2026-11-26T18:00:00Z",
                  conflicts: ["oven is needed"],
                },
              ],
            },
          ],
        },
      })
    );

    const tl = await api.getEventTimeline(3);

    const body = lastBody();
    expect(body.query).toContain("eventTimeline");
    expect(body.variables.id).toBe("3");
    expect(tl.foodEventID).toBe(3);
    expect(tl.warnings).toEqual(["appliance conflict: oven"]);
    expect(tl.recipes).toHaveLength(1);
    const r = tl.recipes[0];
    expect(r.eventRecipeID).toBe(9);
    expect(r.startBy).toBe("2026-11-26T17:30:00Z");
    const s = r.steps[0];
    expect(s.scheduledMinutes).toBe(30);
    expect(s.estimated).toBe(true);
    expect(s.conflicts).toEqual(["oven is needed"]);
  });

  it("getEventTimeline throws 404 when the event is null", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ eventTimeline: null }));

    await expect(api.getEventTimeline(3)).rejects.toThrow(ApiError);
  });
});

describe("recipe step timing round-trip", () => {
  const gqlRecipe = {
    id: "5",
    name: "Casserole",
    description: null,
    servings: 4,
    prepTimeMinutes: null,
    cookTimeMinutes: null,
    items: [],
    steps: [
      {
        stepNumber: 1,
        instruction: "Mix",
        durationMinutes: 10,
        stepType: "prep",
        isPassive: false,
        dependsOnStepNumber: null,
        appliance: "mixer",
      },
      {
        stepNumber: 2,
        instruction: "Bake",
        durationMinutes: 45,
        stepType: "cook",
        isPassive: true,
        dependsOnStepNumber: 1,
        appliance: "oven",
      },
    ],
    isFavorite: false,
    selectionCount: 0,
    personalSelectionCount: 0,
    myRating: null,
    averageRating: null,
    ratingCount: 0,
  };

  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("maps timing fields off the wire", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe }));

    const recipe = await api.getRecipe(5);

    const [s1, s2] = recipe.recipeSteps!;
    expect(s1.durationMinutes).toBe(10);
    expect(s1.stepType).toBe("prep");
    expect(s1.isPassive).toBe(false);
    expect(s1.appliance).toBe("mixer");
    expect(s2.isPassive).toBe(true);
    expect(s2.dependsOnStepNumber).toBe(1);
  });

  it("preserves timing fields when editing an unrelated step", async () => {
    // getRecipe for the fetch-modify-write, then the updateRecipe call.
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe }))
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe }))
      .mockResolvedValueOnce(mockGraphQL({ updateRecipe: gqlRecipe }));

    await api.updateRecipeStep(5, 2, {
      stepNumber: 2,
      instruction: "Bake longer",
    });

    const body = lastBody();
    const steps = body.variables.input.steps as Record<string, unknown>[];
    // Step 1 keeps its timing fields even though only step 2 changed.
    expect(steps[0]).toMatchObject({
      durationMinutes: 10,
      stepType: "prep",
      appliance: "mixer",
    });
    // The caller replaces the step wholesale — omitted timing fields
    // clear, matching the editor's full-form submit.
    expect(steps[1]).toMatchObject({
      instruction: "Bake longer",
      durationMinutes: null,
      isPassive: false,
    });
  });

  it("keeps timing fields the caller passes through", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe }))
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe }))
      .mockResolvedValueOnce(mockGraphQL({ updateRecipe: gqlRecipe }));

    await api.updateRecipeStep(5, 2, {
      stepNumber: 2,
      instruction: "Bake longer",
      durationMinutes: 50,
      stepType: "cook",
      isPassive: true,
      dependsOnStepNumber: 1,
      appliance: "oven",
    });

    const body = lastBody();
    const steps = body.variables.input.steps as Record<string, unknown>[];
    expect(steps[1]).toMatchObject({
      instruction: "Bake longer",
      durationMinutes: 50,
      isPassive: true,
      dependsOnStepNumber: 1,
      appliance: "oven",
    });
  });

  it("preserves timing fields when deleting a step", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe }))
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe }))
      .mockResolvedValueOnce(
        mockGraphQL({ updateRecipe: { ...gqlRecipe, steps: [gqlRecipe.steps[0]] } })
      );

    await api.deleteRecipeStep(5, 2);

    const body = lastBody();
    const steps = body.variables.input.steps as Record<string, unknown>[];
    expect(steps).toHaveLength(1);
    expect(steps[0]).toMatchObject({ durationMinutes: 10, appliance: "mixer" });
  });
});
