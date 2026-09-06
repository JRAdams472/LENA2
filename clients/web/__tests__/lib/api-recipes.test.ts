import {
  api,
  ApiError,
  setAuthTokenGetter,
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
    brand: { id: "2", name: "Acme" },
    upc12: null,
    upc14: null,
    unit: "ea",
    category: gqlCategory,
    nutrients: [],
    flavors: [],
    ...over,
  };
}

function gqlRecipe(over: Record<string, unknown> = {}) {
  return {
    id: "1",
    name: "Pancakes",
    description: "Breakfast",
    servings: 4,
    prepTimeMinutes: 10,
    cookTimeMinutes: 15,
    isFavorite: false,
    items: [
      {
        quantity: 2,
        unit: "cup",
        notes: "sifted",
        isOptional: false,
        item: gqlItem({ id: "5", name: "Flour" }),
      },
    ],
    steps: [
      { stepNumber: 1, instruction: "Mix" },
      { stepNumber: 2, instruction: "Cook" },
    ],
    ...over,
  };
}

function recipesPage(items: object[], totalCount = items.length) {
  return {
    recipes: {
      items,
      pageInfo: { pageNumber: 1, pageSize: 200, totalCount },
    },
  };
}

describe("api client: recipes", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getRecipes paginates through all pages and maps rows", async () => {
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL({
          recipes: {
            items: [gqlRecipe({ id: "1", name: "Pancakes" })],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 201 },
          },
        })
      )
      .mockResolvedValueOnce(
        mockGraphQL({
          recipes: {
            items: [gqlRecipe({ id: "2", name: "Soup", items: null, steps: null })],
            pageInfo: { pageNumber: 2, pageSize: 200, totalCount: 201 },
          },
        })
      );

    const recipes = await api.getRecipes();

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(recipes).toHaveLength(2);
    expect(recipes[0].recipeName).toBe("Pancakes");
    expect(recipes[0].recipeItems?.[0].itemID).toBe(5);
    expect(recipes[0].recipeItems?.[0].itemName).toBe("Flour");
    expect(recipes[0].recipeItems?.[0].itemBrand).toBe("Acme");
    expect(recipes[0].recipeItems?.[0].notes).toBe("sifted");
    expect(recipes[0].recipeSteps?.[1].instruction).toBe("Cook");
    expect(recipes[1].recipeItems).toEqual([]);
  });

  it("getRecipes maps a recipe item with a null item", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(
        recipesPage([
          gqlRecipe({
            items: [
              {
                quantity: 1,
                unit: "ea",
                notes: null,
                isOptional: true,
                item: null,
              },
            ],
          }),
        ])
      )
    );

    const recipes = await api.getRecipes();
    const ri = recipes[0].recipeItems?.[0];
    expect(ri?.itemID).toBe(0);
    expect(ri?.itemName).toBeNull();
    expect(ri?.itemBrand).toBeNull();
    expect(ri?.item).toBeNull();
    expect(ri?.isOptional).toBe(true);
  });

  it("getRecipesPaged hits the server when no filters are given", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        recipes: {
          items: [gqlRecipe()],
          pageInfo: { pageNumber: 2, pageSize: 10, totalCount: 25 },
        },
      })
    );

    const result = await api.getRecipesPaged(2, 10);

    expect(lastRequestBody().variables).toEqual({ page: 2, pageSize: 10 });
    expect(result.totalPages).toBe(3);
    expect(result.items[0].recipeID).toBe(1);
  });

  it("getRecipesPaged filters client-side when search is given", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(
        recipesPage([
          gqlRecipe({ id: "1", name: "Pancakes" }),
          gqlRecipe({ id: "2", name: "Soup", isFavorite: true }),
        ])
      )
    );

    const result = await api.getRecipesPaged(1, 10, "pan");

    expect(result.items).toHaveLength(1);
    expect(result.items[0].recipeName).toBe("Pancakes");
  });

  it("getRecipesPaged filters by isFavorite", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(
        recipesPage([
          gqlRecipe({ id: "1", name: "Pancakes" }),
          gqlRecipe({ id: "2", name: "Soup", isFavorite: true }),
        ])
      )
    );

    const result = await api.getRecipesPaged(1, 10, undefined, true);

    expect(result.items).toHaveLength(1);
    expect(result.items[0].recipeName).toBe("Soup");
  });

  it("getRecipe fetches a single recipe", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }));

    const recipe = await api.getRecipe(1);

    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(recipe.recipeID).toBe(1);
    expect(recipe.servings).toBe(4);
    expect(recipe.isActive).toBe(true);
  });

  it("getRecipe throws ApiError 404 when missing", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ recipe: null }));

    const error = await api.getRecipe(9).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(404);
    expect((error as ApiError).message).toContain("Recipe 9 not found");
  });

  it("createRecipe posts the mapped input", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ createRecipe: gqlRecipe() })
    );

    const recipe = await api.createRecipe({
      recipeID: 0,
      recipeName: "Pancakes",
      description: "Breakfast",
      servings: 4,
      prepTimeMinutes: 10,
      cookTimeMinutes: 15,
      isActive: true,
      isFavorite: false,
      selectionCount: 0,
      personalSelectionCount: 0,
      recipeItems: [
        {
          recipeID: 0,
          itemID: 5,
          quantity: 2,
          unitOfMeasure: "cup",
          notes: "sifted",
          isOptional: false,
        },
      ],
      recipeSteps: [
        {
          recipeStepID: 1,
          recipeID: 0,
          stepNumber: 1,
          instruction: "Mix",
          createdBy: "",
          createDate: "",
          lastUpdatedBy: null,
          lastUpdatedDate: null,
        },
      ],
    });

    expect(lastRequestBody().variables).toEqual({
      input: {
        name: "Pancakes",
        description: "Breakfast",
        servings: 4,
        prepTimeMinutes: 10,
        cookTimeMinutes: 15,
        items: [
          {
            itemId: "5",
            quantity: 2,
            unit: "cup",
            notes: "sifted",
            isOptional: false,
          },
        ],
        steps: [{ stepNumber: 1, instruction: "Mix" }],
      },
    });
    expect(recipe.recipeName).toBe("Pancakes");
  });

  it("updateRecipe fetches then posts the merged input", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(
        mockGraphQL({ updateRecipe: gqlRecipe({ name: "Waffles" }) })
      );

    const recipe = await api.updateRecipe(1, { recipeName: "Waffles" });

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(lastRequestBody().variables.id).toBe("1");
    expect(lastRequestBody().variables.input.name).toBe("Waffles");
    expect(lastRequestBody().variables.input.items).toHaveLength(1);
    expect(recipe.recipeName).toBe("Waffles");
  });

  it("deleteRecipe posts the id and resolves null", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ deleteRecipe: true }));

    const result = await api.deleteRecipe(1);
    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(result).toBeNull();
  });

  it("getRecipeItems returns the recipe's items", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }));

    const items = await api.getRecipeItems(1);
    expect(items).toHaveLength(1);
    expect(items[0].itemID).toBe(5);
  });

  it("addRecipeItem appends the item and returns it", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      // updateRecipe re-fetches the recipe before posting.
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(
        mockGraphQL({
          updateRecipe: gqlRecipe({
            items: [
              {
                quantity: 2,
                unit: "cup",
                notes: "sifted",
                isOptional: false,
                item: gqlItem({ id: "5", name: "Flour" }),
              },
              {
                quantity: 1,
                unit: "ea",
                notes: null,
                isOptional: true,
                item: gqlItem({ id: "8", name: "Egg" }),
              },
            ],
          }),
        })
      );

    const item = await api.addRecipeItem(1, {
      itemId: 8,
      portion: 1,
      unit: "ea",
      isOptional: true,
    });

    expect(lastRequestBody().variables.input.items).toHaveLength(2);
    expect(lastRequestBody().variables.input.items[1]).toEqual({
      itemId: "8",
      quantity: 1,
      unit: "ea",
      notes: null,
      isOptional: true,
    });
    expect(item.itemID).toBe(8);
  });

  it("addRecipeItem falls back when the updated recipe lacks the item", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(
        mockGraphQL({ updateRecipe: gqlRecipe({ items: [] }) })
      );

    const item = await api.addRecipeItem(1, {
      itemId: 8,
      portion: 3,
      unit: null,
      isOptional: false,
    });

    expect(item.itemID).toBe(8);
    expect(item.quantity).toBe(3);
    expect(item.unitOfMeasure).toBeNull();
  });

  it("removeRecipeItem filters the item out and updates", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(
        mockGraphQL({ updateRecipe: gqlRecipe({ items: [] }) })
      );

    await api.removeRecipeItem(1, 5);

    expect(lastRequestBody().variables.input.items).toHaveLength(0);
  });

  it("getRecipeSteps returns the recipe's steps", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }));

    const steps = await api.getRecipeSteps(1);
    expect(steps).toHaveLength(2);
    expect(steps[0].instruction).toBe("Mix");
  });

  it("addRecipeStep appends a step and returns it", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(
        mockGraphQL({ updateRecipe: gqlRecipe() })
      );

    const step = await api.addRecipeStep(1, {
      stepNumber: 3,
      instruction: "Serve",
    });

    expect(lastRequestBody().variables.input.steps).toEqual([
      { stepNumber: 1, instruction: "Mix" },
      { stepNumber: 2, instruction: "Cook" },
      { stepNumber: 3, instruction: "Serve" },
    ]);
    expect(step.stepNumber).toBe(3);
    expect(step.instruction).toBe("Serve");
    expect(step.recipeID).toBe(1);
  });

  it("updateRecipeStep replaces the matching step", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(
        mockGraphQL({ updateRecipe: gqlRecipe() })
      );

    const step = await api.updateRecipeStep(1, 2, {
      stepNumber: 2,
      instruction: "Bake",
    });

    expect(lastRequestBody().variables.input.steps).toEqual([
      { stepNumber: 1, instruction: "Mix" },
      { stepNumber: 2, instruction: "Bake" },
    ]);
    expect(step.instruction).toBe("Bake");
  });

  it("deleteRecipeStep removes the matching step", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(mockGraphQL({ recipe: gqlRecipe() }))
      .mockResolvedValueOnce(
        mockGraphQL({ updateRecipe: gqlRecipe() })
      );

    await api.deleteRecipeStep(1, 1);

    expect(lastRequestBody().variables.input.steps).toEqual([
      { stepNumber: 2, instruction: "Cook" },
    ]);
  });
});
