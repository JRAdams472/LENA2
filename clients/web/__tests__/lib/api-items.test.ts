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

const emptyUserItemsPage = {
  userItems: {
    items: [],
    pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 },
  },
};

const gqlCategory = {
  id: "1",
  name: "Dairy",
  description: "Milk products",
  isActive: true,
};

function gqlItem(over: Record<string, unknown> = {}) {
  return {
    id: "1",
    name: "Milk",
    brand: { id: "2", name: "Acme" },
    upc12: "012345678901",
    upc14: null,
    unit: "ea",
    category: gqlCategory,
    nutrients: [
      {
        amount: 120,
        nutrient: { id: "5", name: "Calories", unit: "kcal" },
      },
    ],
    flavors: [
      {
        intensity: 3,
        flavor: { id: "7", name: "Creamy", isActive: true },
      },
    ],
    ...over,
  };
}

function itemsPage(items: object[], totalCount = items.length) {
  return {
    items: {
      items,
      pageInfo: { pageNumber: 1, pageSize: 200, totalCount },
    },
  };
}

describe("api client: items", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getItems merges user item prefs into items", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL(itemsPage([gqlItem()])))
      .mockResolvedValueOnce(
        mockGraphQL({
          userItems: {
            items: [
              {
                id: "9",
                item: { id: "1" },
                currentQty: 4,
                minQty: 1,
                purchaseAt: "2026-09-01",
                expiresAt: "2026-10-01",
                notes: "keep cold",
                isFavorite: true,
              },
            ],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 },
          },
        })
      );

    const items = await api.getItems();

    expect(items).toHaveLength(1);
    const item = items[0];
    expect(item.itemID).toBe(1);
    expect(item.brand).toBe("Acme");
    expect(item.currentQuantity).toBe(4);
    expect(item.minQuantity).toBe(1);
    expect(item.purchaseDate).toBe("2026-09-01");
    expect(item.expiryDate).toBe("2026-10-01");
    expect(item.notes).toBe("keep cold");
    expect(item.isFavorite).toBe(true);
    expect(item.category?.categoryName).toBe("Dairy");
    expect(item.foodNutrients?.[0].nutrientType?.nutrientName).toBe(
      "Calories"
    );
    expect(item.foodNutrients?.[0].amountPerServing).toBe(120);
    expect(item.foodFlavors?.[0].intensityScore).toBe(3);
    expect(item.foodFlavors?.[0].flavorProfile?.flavorName).toBe("Creamy");
  });

  it("getItems paginates through all item pages", async () => {
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL({
          items: {
            items: [gqlItem({ id: "1", name: "Milk" })],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 2 },
          },
        })
      )
      .mockResolvedValueOnce(mockGraphQL(emptyUserItemsPage))
      .mockResolvedValueOnce(
        mockGraphQL({
          items: {
            items: [gqlItem({ id: "2", name: "Bread" })],
            pageInfo: { pageNumber: 2, pageSize: 200, totalCount: 2 },
          },
        })
      );

    const items = await api.getItems();

    expect(mockFetch).toHaveBeenCalledTimes(3);
    expect(items.map((i) => i.name)).toEqual(["Milk", "Bread"]);
    expect(items[1].isFavorite).toBe(false);
    expect(items[1].currentQuantity).toBe(0);
  });

  it("getItems handles an item with null brand and category", async () => {
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL(
          itemsPage([
            gqlItem({ brand: null, category: null, nutrients: null, flavors: null }),
          ])
        )
      )
      .mockResolvedValueOnce(mockGraphQL(emptyUserItemsPage));

    const items = await api.getItems();

    expect(items[0].brand).toBeNull();
    expect(items[0].category).toBeNull();
    expect(items[0].categoryID).toBe(0);
    expect(items[0].foodNutrients).toEqual([]);
  });

  it("getItems maps non-numeric ids to 0", async () => {
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL(itemsPage([gqlItem({ id: "abc" })]))
      )
      .mockResolvedValueOnce(mockGraphQL(emptyUserItemsPage));

    const items = await api.getItems();
    expect(items[0].itemID).toBe(0);
  });

  it("getItemsPaged applies search, brand, inStock and favorite filters", async () => {
    const rows = [
      gqlItem({ id: "1", name: "Whole Milk", brand: { id: "1", name: "Acme" } }),
      gqlItem({ id: "2", name: "Skim Milk", brand: { id: "2", name: "Beta" } }),
      gqlItem({ id: "3", name: "Bread", brand: null }),
    ];
    mockFetch
      .mockResolvedValueOnce(mockGraphQL(itemsPage(rows)))
      .mockResolvedValueOnce(
        mockGraphQL({
          userItems: {
            items: [
              {
                id: "9",
                item: { id: "1" },
                currentQty: 2,
                minQty: null,
                purchaseAt: null,
                expiresAt: null,
                notes: null,
                isFavorite: true,
              },
            ],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 },
          },
        })
      );

    const result = await api.getItemsPaged(1, 10, "milk", "acme", true, true);

    expect(result.items).toHaveLength(1);
    expect(result.items[0].name).toBe("Whole Milk");
    expect(result.totalCount).toBe(1);
    expect(result.totalPages).toBe(1);
  });

  it("getItemsPaged slices pages with pagedSlice math", async () => {
    const rows = [
      gqlItem({ id: "1", name: "A" }),
      gqlItem({ id: "2", name: "B" }),
      gqlItem({ id: "3", name: "C" }),
    ];
    mockFetch
      .mockResolvedValueOnce(mockGraphQL(itemsPage(rows)))
      .mockResolvedValueOnce(mockGraphQL(emptyUserItemsPage));

    const result = await api.getItemsPaged(2, 2);

    expect(result.items.map((i) => i.name)).toEqual(["C"]);
    expect(result.pageNumber).toBe(2);
    expect(result.pageSize).toBe(2);
    expect(result.totalCount).toBe(3);
    expect(result.totalPages).toBe(2);
  });

  it("searchItems filters by term and brand and respects the limit", async () => {
    const rows = [
      gqlItem({ id: "1", name: "Milk", brand: { id: "1", name: "Acme" } }),
      gqlItem({ id: "2", name: "Milk", brand: { id: "2", name: "Beta" } }),
      gqlItem({ id: "3", name: "Bread", brand: null }),
    ];
    mockFetch
      .mockResolvedValueOnce(mockGraphQL(itemsPage(rows)))
      .mockResolvedValueOnce(mockGraphQL(emptyUserItemsPage));

    const result = await api.searchItems("milk", "acme", 5);
    expect(result).toHaveLength(1);
    expect(result[0].brand).toBe("Acme");
  });

  it("searchItems with no brand filter returns matches up to the limit", async () => {
    const rows = [
      gqlItem({ id: "1", name: "Milk 1" }),
      gqlItem({ id: "2", name: "Milk 2" }),
    ];
    mockFetch
      .mockResolvedValueOnce(mockGraphQL(itemsPage(rows)))
      .mockResolvedValueOnce(mockGraphQL(emptyUserItemsPage));

    const result = await api.searchItems("milk", undefined, 1);
    expect(result).toHaveLength(1);
  });

  it("getItem fetches a single item and maps it", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ item: gqlItem() }));

    const item = await api.getItem(1);

    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(item.itemID).toBe(1);
    expect(item.name).toBe("Milk");
    expect(item.upc12).toBe("012345678901");
  });

  it("getItem throws ApiError 404 when the item is missing", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ item: null }));

    const error = await api.getItem(99).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(404);
    expect((error as ApiError).message).toContain("Item 99 not found");
  });

  it("createItem posts the input and maps the result", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ createItem: gqlItem() })
    );

    const item = await api.createItem({
      itemID: 0,
      name: "Milk",
      brand: null,
      upc12: "012345678901",
      upc14: null,
      categoryID: 1,
      unit: "ea",
      currentQuantity: 0,
      minQuantity: null,
      purchaseDate: null,
      expiryDate: null,
      notes: null,
      isFavorite: false,
      category: null,
      foodNutrients: null,
      foodFlavors: null,
      selectionCount: 0,
      personalSelectionCount: 0,
    });

    expect(lastRequestBody().variables).toEqual({
      input: {
        name: "Milk",
        brandId: null,
        upc12: "012345678901",
        upc14: null,
        categoryId: "1",
        unit: "ea",
      },
    });
    expect(item.itemID).toBe(1);
  });

  it("updateItem sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ updateItem: gqlItem({ name: "Skim" }) })
    );

    const item = await api.updateItem(1, { name: "Skim", unit: "gal" });

    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { name: "Skim", unit: "gal" },
    });
    expect(item.name).toBe("Skim");
  });

  it("updateItem maps categoryID to categoryId", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ updateItem: gqlItem() }));

    await api.updateItem(1, { categoryID: 4, upc12: "1", upc14: "2" });

    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { categoryId: "4", upc12: "1", upc14: "2" },
    });
  });

  it("deleteItem posts the id and resolves null", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ deleteItem: true }));

    const result = await api.deleteItem(1);
    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(result).toBeNull();
  });

  it("changeItemCategory issues an updateItem mutation", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ updateItem: gqlItem() }));

    await api.changeItemCategory(1, 7);
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { categoryId: "7" },
    });
  });

  it("setItemUPC12 and setItemUPC14 issue updateItem mutations", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ updateItem: gqlItem() }));
    await api.setItemUPC12(1, "111");
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { upc12: "111" },
    });

    mockFetch.mockResolvedValueOnce(mockGraphQL({ updateItem: gqlItem() }));
    await api.setItemUPC14(1, "222");
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { upc14: "222" },
    });
  });
});

describe("api client: food flavors and nutrients", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getFoodFlavors flattens item flavors and attaches the item", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL(itemsPage([gqlItem()])));

    const flavors = await api.getFoodFlavors();

    expect(flavors).toHaveLength(1);
    expect(flavors[0].foodId).toBe(1);
    expect(flavors[0].flavorId).toBe(7);
    expect(flavors[0].item?.name).toBe("Milk");
  });

  it("getFoodNutrients flattens item nutrients", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL(itemsPage([gqlItem()])));

    const nutrients = await api.getFoodNutrients();

    expect(nutrients).toHaveLength(1);
    expect(nutrients[0].foodId).toBe(1);
    expect(nutrients[0].nutrientId).toBe(5);
    expect(nutrients[0].amountPerServing).toBe(120);
  });

  it("createFoodFlavor posts ids and intensity", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        addFoodFlavor: {
          intensity: 4,
          flavor: { id: "7", name: "Creamy", isActive: true },
        },
      })
    );

    const flavor = await api.createFoodFlavor({
      foodId: 1,
      flavorId: 7,
      intensityScore: 4,
      item: null,
      flavorProfile: null,
    });

    expect(lastRequestBody().variables).toEqual({
      input: { itemId: "1", flavorId: "7", intensity: 4 },
    });
    expect(flavor.flavorId).toBe(7);
    expect(flavor.intensityScore).toBe(4);
  });

  it("updateFoodFlavor removes then re-adds the flavor", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ removeFoodFlavor: true }))
      .mockResolvedValueOnce(
        mockGraphQL({
          addFoodFlavor: {
            intensity: 2,
            flavor: { id: "7", name: "Creamy", isActive: true },
          },
        })
      );

    const flavor = await api.updateFoodFlavor(1, 7, { intensityScore: 2 });

    expect(lastRequestBody(-2).variables).toEqual({
      itemId: "1",
      flavorId: "7",
    });
    expect(lastRequestBody().variables).toEqual({
      input: { itemId: "1", flavorId: "7", intensity: 2 },
    });
    expect(flavor.intensityScore).toBe(2);
  });

  it("updateFoodFlavor defaults intensity to 0 when omitted", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ removeFoodFlavor: true }))
      .mockResolvedValueOnce(
        mockGraphQL({
          addFoodFlavor: {
            intensity: 0,
            flavor: { id: "7", name: "Creamy", isActive: true },
          },
        })
      );

    await api.updateFoodFlavor(1, 7, {});
    expect(lastRequestBody().variables.input.intensity).toBe(0);
  });

  it("deleteFoodFlavor posts item and flavor ids", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ removeFoodFlavor: true })
    );

    const result = await api.deleteFoodFlavor(1, 7);
    expect(lastRequestBody().variables).toEqual({
      itemId: "1",
      flavorId: "7",
    });
    expect(result).toBeNull();
  });

  it("createFoodNutrient posts ids and amount", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        addFoodNutrient: {
          amount: 50,
          nutrient: { id: "5", name: "Calories", unit: "kcal" },
        },
      })
    );

    const nutrient = await api.createFoodNutrient({
      foodId: 1,
      nutrientId: 5,
      amountPerServing: 50,
      nutrientType: null,
    });

    expect(lastRequestBody().variables).toEqual({
      input: { itemId: "1", nutrientId: "5", amount: 50 },
    });
    expect(nutrient.nutrientId).toBe(5);
    expect(nutrient.nutrientType?.unitOfMeasure).toBe("kcal");
  });

  it("updateFoodNutrient removes then re-adds the nutrient", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ removeFoodNutrient: true }))
      .mockResolvedValueOnce(
        mockGraphQL({
          addFoodNutrient: {
            amount: 75,
            nutrient: { id: "5", name: "Calories", unit: "kcal" },
          },
        })
      );

    const nutrient = await api.updateFoodNutrient(1, 5, {
      amountPerServing: 75,
    });

    expect(lastRequestBody(-2).variables).toEqual({
      itemId: "1",
      nutrientId: "5",
    });
    expect(nutrient.amountPerServing).toBe(75);
  });

  it("updateFoodNutrient defaults amount to 0 when omitted", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ removeFoodNutrient: true }))
      .mockResolvedValueOnce(
        mockGraphQL({
          addFoodNutrient: {
            amount: 0,
            nutrient: { id: "5", name: "Calories", unit: "kcal" },
          },
        })
      );

    await api.updateFoodNutrient(1, 5, {});
    expect(lastRequestBody().variables.input.amount).toBe(0);
  });

  it("deleteFoodNutrient posts item and nutrient ids", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ removeFoodNutrient: true })
    );

    const result = await api.deleteFoodNutrient(1, 5);
    expect(lastRequestBody().variables).toEqual({
      itemId: "1",
      nutrientId: "5",
    });
    expect(result).toBeNull();
  });
});

describe("api client: inventory deletes and creates", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("createFlavorProfile posts the name", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createFlavorProfile: { id: "3", name: "Sour", isActive: true },
      })
    );

    const profile = await api.createFlavorProfile({
      flavorId: 0,
      flavorName: "Sour",
      isActive: true,
    });
    expect(lastRequestBody().variables).toEqual({
      input: { name: "Sour" },
    });
    expect(profile.flavorId).toBe(3);
  });

  it("deleteFlavorProfile posts the id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ deleteFlavorProfile: true })
    );

    const result = await api.deleteFlavorProfile(3);
    expect(lastRequestBody().variables).toEqual({ id: "3" });
    expect(result).toBeNull();
  });

  it("deleteNutrientType posts the id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ deleteNutrientType: true })
    );

    const result = await api.deleteNutrientType(5);
    expect(lastRequestBody().variables).toEqual({ id: "5" });
    expect(result).toBeNull();
  });
});
