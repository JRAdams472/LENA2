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

const DEFAULT_PAGE = { pageNumber: 1, pageSize: 200 };

function itemsPage(
  items: object[],
  totalCount = items.length,
  pageInfo: { pageNumber: number; pageSize: number } = DEFAULT_PAGE
) {
  return {
    items: {
      items,
      pageInfo: { ...pageInfo, totalCount },
    },
  };
}

function gqlUserItem(itemId: string, over: Record<string, unknown> = {}) {
  return {
    id: "9",
    item: gqlItem({ id: itemId }),
    currentQty: 4,
    minQty: 1,
    purchaseAt: "2026-09-01",
    expiresAt: "2026-10-01",
    notes: "keep cold",
    isFavorite: true,
    ...over,
  };
}

function userItemsPage(items: object[], totalCount = items.length) {
  return {
    userItems: {
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

  it("getItemsPaged requests a server-side page and merges user prefs", async () => {
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL(itemsPage([gqlItem()], 57, { pageNumber: 2, pageSize: 10 }))
      )
      .mockResolvedValueOnce(
        mockGraphQL(userItemsPage([gqlUserItem("1")]))
      );

    const result = await api.getItemsPaged(2, 10, "milk", 3);

    expect(mockFetch).toHaveBeenCalledTimes(2);
    const body = lastRequestBody(-2);
    expect(body.query).toContain("items(page: $page");
    expect(body.query).toContain("brandId: $brandId");
    expect(body.variables).toEqual({
      page: 2,
      pageSize: 10,
      search: "milk",
      brandId: "3",
    });
    const item = result.items[0];
    expect(item.itemID).toBe(1);
    expect(item.brand).toBe("Acme");
    expect(item.currentQuantity).toBe(4);
    expect(item.minQuantity).toBe(1);
    expect(item.purchaseDate).toBe("2026-09-01");
    expect(item.expiryDate).toBe("2026-10-01");
    expect(item.notes).toBe("keep cold");
    expect(item.isFavorite).toBe(true);
    expect(item.category?.categoryName).toBe("Dairy");
    expect(result.totalCount).toBe(57);
    expect(result.totalPages).toBe(6);
    expect(result.pageNumber).toBe(2);
  });

  it("getItemsPaged filters the pantry path via userItems", async () => {
    const userItems = [
      gqlUserItem("1", {
        item: gqlItem({ id: "1", name: "Whole Milk", brand: { id: "1", name: "Acme" } }),
        currentQty: 2,
        isFavorite: true,
      }),
      gqlUserItem("2", {
        item: gqlItem({ id: "2", name: "Skim Milk", brand: { id: "2", name: "Beta" } }),
        currentQty: 0,
        isFavorite: false,
      }),
      gqlUserItem("3", {
        item: gqlItem({ id: "3", name: "Bread", brand: null }),
        currentQty: 5,
        isFavorite: false,
      }),
    ];
    mockFetch.mockResolvedValueOnce(mockGraphQL(userItemsPage(userItems)));

    const result = await api.getItemsPaged(1, 10, "milk", 1, true, true);

    expect(mockFetch).toHaveBeenCalledTimes(1);
    const body = lastRequestBody();
    expect(body.query).toContain("userItems(page: $page");
    expect(body.variables.search).toBe("milk");
    expect(result.items).toHaveLength(1);
    expect(result.items[0].name).toBe("Whole Milk");
    expect(result.totalCount).toBe(1);
  });

  it("getItemsPaged handles an item with null brand and category", async () => {
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL(
          itemsPage([
            gqlItem({ brand: null, category: null, nutrients: null, flavors: null }),
          ])
        )
      )
      .mockResolvedValueOnce(mockGraphQL(emptyUserItemsPage));

    const result = await api.getItemsPaged(1, 25);

    expect(result.items[0].brand).toBeNull();
    expect(result.items[0].category).toBeNull();
    expect(result.items[0].categoryID).toBe(0);
    expect(result.items[0].foodNutrients).toEqual([]);
  });

  it("getItemsPaged maps non-numeric ids to 0", async () => {
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL(itemsPage([gqlItem({ id: "abc" })]))
      )
      .mockResolvedValueOnce(mockGraphQL(emptyUserItemsPage));

    const result = await api.getItemsPaged(1, 25);
    expect(result.items[0].itemID).toBe(0);
  });

  it("searchItems issues a ranked server-side query", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(itemsPage([gqlItem({ id: "1", name: "Milk" })]))
    );

    const result = await api.searchItems("  milk  ", 7, 25);

    expect(mockFetch).toHaveBeenCalledTimes(1);
    const body = lastRequestBody();
    expect(body.query).toContain("items(page: 1");
    expect(body.variables).toEqual({ search: "milk", brandId: "7", limit: 25 });
    expect(result).toHaveLength(1);
    expect(result[0].name).toBe("Milk");
  });

  it("searchItems omits null filters", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(itemsPage([gqlItem({ id: "1" })]))
    );

    const result = await api.searchItems("", undefined, 10);

    expect(lastRequestBody().variables).toEqual({
      search: null,
      brandId: null,
      limit: 10,
    });
    expect(result).toHaveLength(1);
  });

  it("getItemsByIds fetches each item and dedupes ids", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ item: gqlItem({ id: "1", name: "Milk" }) }))
      .mockResolvedValueOnce(mockGraphQL({ item: gqlItem({ id: "2", name: "Bread" }) }));

    const items = await api.getItemsByIds([1, 2, 1]);

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(items.map((i) => i.name)).toEqual(["Milk", "Bread"]);
  });

  it("getItemsByIds skips items that fail to load", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ item: null }))
      .mockResolvedValueOnce(mockGraphQL({ item: gqlItem({ id: "2", name: "Bread" }) }));

    const items = await api.getItemsByIds([99, 2]);

    expect(items).toHaveLength(1);
    expect(items[0].name).toBe("Bread");
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
