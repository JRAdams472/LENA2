import {
  api,
  asEntity,
  ApiError,
  setAuthTokenGetter,
  setOnUnauthorized,
} from "@/lib/api";

const mockFetch = global.fetch as jest.Mock;

function mockGraphQL(data: object | null, status = 200) {
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

function mockGraphQLErrors(
  errors: { message: string; code?: string }[],
  status = 200
) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: {
      get: (name: string) =>
        name === "content-type" ? "application/json" : null,
    },
    json: async () => ({
      errors: errors.map((e) => ({
        message: e.message,
        extensions: e.code ? { code: e.code } : undefined,
      })),
    }),
  };
}

function lastRequestBody(callIndex = -1) {
  const calls = mockFetch.mock.calls;
  const [, init] = calls[calls.length + callIndex];
  return JSON.parse((init as RequestInit).body as string);
}

const gqlCategory = {
  id: "1",
  name: "Dairy",
  description: null,
  isActive: true,
};

function gqlItem(over: Record<string, unknown> = {}) {
  return {
    id: "1",
    name: "Milk",
    brand: null,
    upc12: null,
    upc14: null,
    unit: "ea",
    category: gqlCategory,
    nutrients: [],
    flavors: [],
    ...over,
  };
}

function gqlSlot(over: Record<string, unknown> = {}) {
  return {
    id: "10",
    dayOfWeek: 1,
    mealType: "dinner",
    recipe: null,
    servings: 2,
    replacementNote: null,
    items: [],
    ...over,
  };
}

function gqlMealPlan(over: Record<string, unknown> = {}) {
  return {
    id: "1",
    name: "Weekly",
    weekStartDate: "2026-09-01",
    isActive: true,
    slots: [],
    ...over,
  };
}

function mealPlansPage(items: object[], totalCount = items.length) {
  return {
    mealPlans: {
      items,
      pageInfo: { pageNumber: 1, pageSize: 200, totalCount },
    },
  };
}

describe("api client: auth and helpers", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getMe maps the user", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        me: { id: "7", email: "a@b.c", displayName: "Ann" },
      })
    );

    const user = await api.getMe();
    expect(user).toEqual({
      userID: 7,
      email: "a@b.c",
      displayName: "Ann",
      externalSubject: null,
      provider: null,
    });
  });

  it("asEntity returns objects and rejects non-objects", () => {
    expect(asEntity<{ a: number }>({ a: 1 })).toEqual({ a: 1 });
    expect(() => asEntity(null)).toThrow(TypeError);
    expect(() => asEntity("x")).toThrow(TypeError);
  });

  it("uses the HTTP status text fallback when the body read fails", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      headers: { get: () => null },
      text: async () => {
        throw new Error("read failed");
      },
    });

    const error = await api.getBrandList().catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).message).toBe("HTTP 500");
  });

  it("calls onUnauthorized on UNAUTHORIZED GraphQL errors", async () => {
    const onUnauthorized = jest.fn();
    setOnUnauthorized(onUnauthorized);
    mockFetch.mockResolvedValueOnce(
      mockGraphQLErrors([{ message: "forbidden", code: "UNAUTHORIZED" }])
    );

    await api.getBrandList().catch(() => undefined);
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
    setOnUnauthorized(() => undefined);
  });

  it("does not call onUnauthorized for other error codes", async () => {
    const onUnauthorized = jest.fn();
    setOnUnauthorized(onUnauthorized);
    mockFetch.mockResolvedValueOnce(
      mockGraphQLErrors([{ message: "bad", code: "BAD_USER_INPUT" }])
    );

    await api.getBrandList().catch(() => undefined);
    expect(onUnauthorized).not.toHaveBeenCalled();
    setOnUnauthorized(() => undefined);
  });

  it("does not call onUnauthorized on non-401 HTTP errors", async () => {
    const onUnauthorized = jest.fn();
    setOnUnauthorized(onUnauthorized);
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 403,
      headers: { get: () => null },
      text: async () => "Forbidden",
    });

    await api.getBrandList().catch(() => undefined);
    expect(onUnauthorized).not.toHaveBeenCalled();
    setOnUnauthorized(() => undefined);
  });
});

describe("api client: meal plans and slots", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getMealPlansPaged maps paged results", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        mealPlans: {
          items: [gqlMealPlan({ slots: [gqlSlot()] })],
          pageInfo: { pageNumber: 1, pageSize: 10, totalCount: 1 },
        },
      })
    );

    const result = await api.getMealPlansPaged(1, 10);

    expect(lastRequestBody().variables).toEqual({ page: 1, pageSize: 10 });
    expect(result.items[0].mealPlanID).toBe(1);
    const slot = result.items[0].mealSlots?.[0];
    expect(slot?.mealSlotID).toBe(10);
    expect(slot?.mealType).toBe(2);
    expect(slot?.recipeID).toBeNull();
    expect(slot?.recipe).toBeNull();
  });

  it("getMealPlan maps slots with recipes, items and meal type variants", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        mealPlan: gqlMealPlan({
          slots: [
            gqlSlot({
              id: "10",
              mealType: "Breakfast",
              recipe: {
                id: "3",
                name: "Eggs",
                description: null,
                servings: null,
                prepTimeMinutes: null,
                cookTimeMinutes: null,
                items: [],
                steps: [],
                isFavorite: false,
              },
              items: [
                {
                  id: "20",
                  item: gqlItem({ id: "5", name: "Egg" }),
                  quantity: 2,
                  unit: "ea",
                  isFromRecipe: true,
                },
                {
                  id: "21",
                  item: null,
                  quantity: 1,
                  unit: "ea",
                  isFromRecipe: false,
                },
              ],
            }),
            gqlSlot({ id: "11", mealType: "3" }),
            gqlSlot({ id: "12", mealType: "brunch" }),
          ],
        }),
      })
    );

    const plan = await api.getMealPlan(1);
    const slots = plan.mealSlots ?? [];

    expect(slots[0].mealType).toBe(0);
    expect(slots[0].recipeID).toBe(3);
    expect(slots[0].recipe?.recipeName).toBe("Eggs");
    expect(slots[0].mealSlotItems?.[0].item?.name).toBe("Egg");
    expect(slots[0].mealSlotItems?.[0].isFromRecipe).toBe(true);
    expect(slots[0].mealSlotItems?.[1].item).toBeNull();
    expect(slots[0].mealSlotItems?.[1].itemID).toBe(0);
    // "3" parses numerically; "brunch" falls back to 0
    expect(slots[1].mealType).toBe(3);
    expect(slots[2].mealType).toBe(0);
  });

  it("getMealPlan throws ApiError 404 when missing", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ mealPlan: null }));

    const error = await api.getMealPlan(9).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(404);
  });

  it("createMealPlan posts name and week fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ createMealPlan: gqlMealPlan() })
    );

    const plan = await api.createMealPlan({
      planName: "Weekly",
      weekStartDate: "2026-09-01",
      weekStartDayOfWeek: 1,
      isActive: true,
    });

    expect(lastRequestBody().variables).toEqual({
      input: {
        name: "Weekly",
        weekStartDate: "2026-09-01",
        weekStartDayOfWeek: 1,
      },
    });
    expect(plan.mealPlanID).toBe(1);
    expect(plan.planName).toBe("Weekly");
  });

  it("updateMealPlan fetches then posts merged fields", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ mealPlan: gqlMealPlan() }))
      .mockResolvedValueOnce(
        mockGraphQL({
          updateMealPlan: gqlMealPlan({ name: "Renamed" }),
        })
      );

    const plan = await api.updateMealPlan(1, { planName: "Renamed" });

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: {
        name: "Renamed",
        weekStartDate: "2026-09-01",
        weekStartDayOfWeek: 0,
      },
    });
    expect(plan.planName).toBe("Renamed");
  });

  it("updateMealPlan uses provided week fields", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ mealPlan: gqlMealPlan() }))
      .mockResolvedValueOnce(
        mockGraphQL({ updateMealPlan: gqlMealPlan() })
      );

    await api.updateMealPlan(1, {
      weekStartDate: "2026-10-01",
      weekStartDayOfWeek: 2,
    });

    expect(lastRequestBody().variables.input).toEqual({
      name: "Weekly",
      weekStartDate: "2026-10-01",
      weekStartDayOfWeek: 2,
    });
  });

  it("deleteMealPlan posts the id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ deleteMealPlan: true })
    );

    const result = await api.deleteMealPlan(1);
    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(result).toBeNull();
  });

  it("getMealSlots returns the plan's slots", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ mealPlan: gqlMealPlan({ slots: [gqlSlot()] }) })
    );

    const slots = await api.getMealSlots(1);
    expect(slots).toHaveLength(1);
    expect(slots[0].mealPlanID).toBe(1);
  });

  it("addMealSlot maps the numeric meal type to a string", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ addMealSlot: gqlSlot() })
    );

    const slot = await api.addMealSlot(1, {
      dayOfWeek: 1,
      mealType: 2,
      recipeID: 3,
      servings: 2,
      replacementNote: null,
      createdBy: "",
      createDate: "",
      lastUpdatedBy: null,
      lastUpdatedDate: null,
    });

    expect(lastRequestBody().variables).toEqual({
      input: {
        mealPlanId: "1",
        dayOfWeek: 1,
        mealType: "Dinner",
        recipeId: "3",
        servings: 2,
        replacementNote: null,
      },
    });
    expect(slot.mealPlanID).toBe(1);
  });

  it("addMealSlot passes a string meal type through and nulls a missing recipe", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ addMealSlot: gqlSlot() })
    );

    await api.addMealSlot(1, {
      dayOfWeek: 0,
      mealType: "lunch" as unknown as number,
      recipeID: null,
      servings: 1,
      replacementNote: "note",
      createdBy: "",
      createDate: "",
      lastUpdatedBy: null,
      lastUpdatedDate: null,
    });

    expect(lastRequestBody().variables.input).toEqual({
      mealPlanId: "1",
      dayOfWeek: 0,
      mealType: "lunch",
      recipeId: null,
      servings: 1,
      replacementNote: "note",
    });
  });

  it("addMealSlot stringifies unknown numeric meal types", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ addMealSlot: gqlSlot() })
    );

    await api.addMealSlot(1, {
      dayOfWeek: 0,
      mealType: 9,
      recipeID: null,
      servings: 1,
      replacementNote: null,
      createdBy: "",
      createDate: "",
      lastUpdatedBy: null,
      lastUpdatedDate: null,
    });

    expect(lastRequestBody().variables.input.mealType).toBe("9");
  });

  it("updateMealSlot deletes then re-adds the slot", async () => {
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL({ mealPlan: gqlMealPlan({ slots: [gqlSlot()] }) })
      )
      .mockResolvedValueOnce(mockGraphQL({ removeMealSlot: true }))
      .mockResolvedValueOnce(
        mockGraphQL({ addMealSlot: gqlSlot({ dayOfWeek: 2 }) })
      );

    const slot = await api.updateMealSlot(1, 10, { dayOfWeek: 2 });

    expect(mockFetch).toHaveBeenCalledTimes(3);
    expect(lastRequestBody(-2).variables).toEqual({ slotId: "10" });
    expect(lastRequestBody().variables.input.dayOfWeek).toBe(2);
    expect(lastRequestBody().variables.input.servings).toBe(2);
    expect(slot.dayOfWeek).toBe(2);
  });

  it("updateMealSlot throws ApiError 404 when the slot is missing", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ mealPlan: gqlMealPlan() })
    );

    const error = await api.updateMealSlot(1, 999, {}).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(404);
  });

  it("deleteMealSlot posts the slot id", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ removeMealSlot: true }));

    await api.deleteMealSlot(1, 10);
    expect(lastRequestBody().variables).toEqual({ slotId: "10" });
  });

  it("getMealSlotItems finds the slot across plans", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(
        mealPlansPage([
          gqlMealPlan({
            slots: [
              gqlSlot({
                id: "10",
                items: [
                  {
                    id: "20",
                    item: gqlItem(),
                    quantity: 1,
                    unit: "ea",
                    isFromRecipe: false,
                  },
                ],
              }),
            ],
          }),
        ])
      )
    );

    const items = await api.getMealSlotItems(10);
    expect(items).toHaveLength(1);
    expect(items[0].mealSlotItemID).toBe(20);
    expect(items[0].itemID).toBe(1);
  });

  it("getMealSlotItems returns empty when the slot is not found", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(mealPlansPage([gqlMealPlan()]))
    );

    const items = await api.getMealSlotItems(999);
    expect(items).toEqual([]);
  });

  it("addMealSlotItem posts the input and maps the result", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        addMealSlotItem: {
          id: "30",
          item: gqlItem({ id: "5" }),
          quantity: 2,
          unit: "ea",
          isFromRecipe: false,
        },
      })
    );

    const item = await api.addMealSlotItem(10, {
      itemID: 5,
      quantity: 2,
      unitOfMeasure: "ea",
      isFromRecipe: false,
      createdBy: "",
      createDate: "",
      lastUpdatedBy: null,
      lastUpdatedDate: null,
    });

    expect(lastRequestBody().variables).toEqual({
      input: {
        slotId: "10",
        itemId: "5",
        quantity: 2,
        unit: "ea",
        isFromRecipe: false,
      },
    });
    expect(item.mealSlotItemID).toBe(30);
    expect(item.mealSlotID).toBe(10);
    expect(item.item?.name).toBe("Milk");
  });

  it("addMealSlotItem defaults the unit to an empty string", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        addMealSlotItem: {
          id: "30",
          item: null,
          quantity: 2,
          unit: "",
          isFromRecipe: false,
        },
      })
    );

    await api.addMealSlotItem(10, {
      itemID: 5,
      quantity: 2,
      unitOfMeasure: null,
      isFromRecipe: false,
      createdBy: "",
      createDate: "",
      lastUpdatedBy: null,
      lastUpdatedDate: null,
    });

    expect(lastRequestBody().variables.input.unit).toBe("");
  });

  it("deleteMealSlotItem posts the item id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ removeMealSlotItem: true })
    );

    await api.deleteMealSlotItem(10, 30);
    expect(lastRequestBody().variables).toEqual({ slotItemId: "30" });
  });
});

describe("api client: grocery list get", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getGroceryList maps the list and its items", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        groceryList: {
          id: "2",
          generatedAt: "2026-09-01T00:00:00Z",
          items: [
            {
              id: "11",
              item: gqlItem({ id: "5", name: "Milk" }),
              manualItemName: null,
              quantityNeeded: 3,
              unitOfMeasure: "ea",
              source: "mealplan",
              isChecked: true,
            },
          ],
        },
      })
    );

    const list = await api.getGroceryList(2);

    expect(lastRequestBody().variables).toEqual({ id: "2" });
    expect(list.groceryListID).toBe(2);
    expect(list.generatedDate).toBe("2026-09-01T00:00:00Z");
    const item = list.groceryListItems?.[0];
    expect(item?.groceryListItemID).toBe(11);
    expect(item?.itemID).toBe(5);
    expect(item?.itemName).toBe("Milk");
    expect(item?.isChecked).toBe(true);
    expect(item?.groceryListID).toBe(2);
  });

  it("getGroceryList throws ApiError 404 when missing", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ groceryList: null }));

    const error = await api.getGroceryList(9).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(404);
    expect((error as ApiError).message).toContain("GroceryList 9 not found");
  });
});
