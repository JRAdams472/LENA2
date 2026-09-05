import {
  api,
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

function lastRequestBody() {
  const [, init] = mockFetch.mock.calls[mockFetch.mock.calls.length - 1];
  return JSON.parse((init as RequestInit).body as string);
}

describe("api client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
    // Default response for the userItems prefs fetch that item-list calls
    // issue in parallel with the items query.
    mockFetch.mockResolvedValue(
      mockGraphQL({
        userItems: {
          items: [],
          pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 },
        },
      })
    );
  });

  it("getItems calls the GraphQL endpoint and returns data", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        items: {
          items: [
            {
              id: "1",
              name: "Milk",
              brand: null,
              upc12: null,
              upc14: null,
              unit: "ea",
              category: { id: "1", name: "Dairy", description: null },
              nutrients: [],
              flavors: [],
            },
          ],
          pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 },
        },
      })
    );

    const result = await api.getItems();

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:5059/graphql",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Accept: "application/json" }),
      })
    );
    expect(result).toHaveLength(1);
    expect(result[0].name).toBe("Milk");
  });

  it("getBottlesPaged calls the GraphQL endpoint and returns data", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        bottles: {
          items: [
            {
              id: "1",
              typeId: "1",
              countryId: "1",
              regionId: "1",
              vineyard: "Napa",
              vintageYear: 2020,
              abv: 13.5,
              acidity: null,
              tanninLevel: null,
              body: null,
              sweetness: null,
              oakIntegration: false,
              bottleSize: "750ml",
              grapeVarieties: [],
              flavorProfiles: [],
            },
          ],
          pageInfo: { pageNumber: 2, pageSize: 50, totalCount: 1 },
        },
      })
    );

    const result = await api.getBottlesPaged(2, 50);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:5059/graphql",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Accept: "application/json" }),
      })
    );
    expect(result.items).toHaveLength(1);
    expect(result.items[0].bottleID).toBe(1);
  });

  it("throws ApiError on non-ok responses", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      headers: { get: () => null },
      text: async () => "Bad Request",
    });

    const error = await api.getItems().catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(400);
    expect((error as ApiError).message).toContain("Bad Request");
  });

  it("omits the Authorization header when no token is set", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        items: {
          items: [],
          pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 },
        },
      })
    );

    await api.getItems();

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:5059/graphql",
      expect.objectContaining({
        headers: expect.not.objectContaining({
          Authorization: expect.any(String),
        }),
      })
    );
  });

  it("adds the Authorization header when a token is set", async () => {
    setAuthTokenGetter(() => "google_id_token");
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        items: {
          items: [],
          pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 },
        },
      })
    );

    await api.getItems();

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:5059/graphql",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer google_id_token",
        }),
      })
    );
  });

  it("throws ApiError with joined messages on GraphQL errors", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQLErrors([
        { message: "first problem" },
        { message: "second problem" },
      ])
    );

    const error = await api.getBrandList().catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).message).toBe(
      "first problem; second problem"
    );
  });

  it("calls onUnauthorized on HTTP 401", async () => {
    const onUnauthorized = jest.fn();
    setOnUnauthorized(onUnauthorized);
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      headers: { get: () => null },
      text: async () => "Unauthorized",
    });

    const error = await api.getBrandList().catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(401);
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it("calls onUnauthorized on UNAUTHENTICATED GraphQL errors", async () => {
    const onUnauthorized = jest.fn();
    setOnUnauthorized(onUnauthorized);
    mockFetch.mockResolvedValueOnce(
      mockGraphQLErrors([{ message: "not signed in", code: "UNAUTHENTICATED" }])
    );

    const error = await api.getBrandList().catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it("throws ApiError when the response contains no data", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL(null));

    const error = await api.getBrandList().catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).message).toContain("no data");
  });
});

describe("api client: brands", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getBrandList maps rows", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ brands: [{ id: "7", name: "Acme" }] })
    );

    const brands = await api.getBrandList();
    expect(brands).toEqual([{ brandID: 7, brandName: "Acme" }]);
  });

  it("getBrands filters by search term", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        brands: [
          { id: "1", name: "Acme" },
          { id: "2", name: "Beta" },
        ],
      })
    );

    const names = await api.getBrands("acm");
    expect(names).toEqual(["Acme"]);
  });

  it("createBrand posts the input and maps the result", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ createBrand: { id: "3", name: "NewCo" } })
    );

    const brand = await api.createBrand({ brandID: 0, brandName: "NewCo" });
    const body = lastRequestBody();
    expect(body.query).toContain("createBrand");
    expect(body.variables).toEqual({ input: { name: "NewCo" } });
    expect(brand.brandID).toBe(3);
  });

  it("updateBrand only sends provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ updateBrand: { id: "3", name: "Renamed" } })
    );

    const brand = await api.updateBrand(3, { brandName: "Renamed" });
    const body = lastRequestBody();
    expect(body.variables).toEqual({
      id: "3",
      input: { name: "Renamed" },
    });
    expect(brand.brandName).toBe("Renamed");
  });

  it("deleteBrand posts the id and resolves null", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ deleteBrand: true }));

    const result = await api.deleteBrand(3);
    const body = lastRequestBody();
    expect(body.query).toContain("deleteBrand");
    expect(body.variables).toEqual({ id: "3" });
    expect(result).toBeNull();
  });
});

describe("api client: categories", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  const row = { id: "1", name: "Dairy", description: "Milk", isActive: true };

  it("getCategories maps rows", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ categories: [row] }));

    const categories = await api.getCategories();
    expect(categories[0].categoryName).toBe("Dairy");
    expect(categories[0].isActive).toBe(true);
  });

  it("getActiveCategories filters out inactive rows", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        categories: [row, { ...row, id: "2", name: "Old", isActive: false }],
      })
    );

    const categories = await api.getActiveCategories();
    expect(categories).toHaveLength(1);
    expect(categories[0].categoryName).toBe("Dairy");
  });

  it("createCategory posts name and description", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ createCategory: row }));

    await api.createCategory({
      categoryID: 0,
      categoryName: "Dairy",
      description: "Milk",
      isActive: true,
    });
    expect(lastRequestBody().variables).toEqual({
      input: { name: "Dairy", description: "Milk" },
    });
  });

  it("updateCategory sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ updateCategory: row }));

    await api.updateCategory(1, { isActive: false });
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { isActive: false },
    });
  });

  it("deleteCategory posts the id", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ deleteCategory: true }));

    await api.deleteCategory(1);
    expect(lastRequestBody().variables).toEqual({ id: "1" });
  });
});

describe("api client: wine reference data", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getCountries maps isoCode and defaults", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        countries: [
          { id: "1", name: "France", isoCode: "FR", description: null },
        ],
      })
    );

    const countries = await api.getCountries();
    expect(countries[0].countryName).toBe("France");
    expect(countries[0].isoCode).toBe("FR");
  });

  it("createCountry posts name, isoCode, description", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createCountry: {
          id: "1",
          name: "France",
          isoCode: "FR",
          description: null,
        },
      })
    );

    await api.createCountry({
      countryID: 0,
      countryName: "France",
      isoCode: "FR",
      description: null,
      isActive: true,
      regions: [],
      bottles: [],
    });
    expect(lastRequestBody().variables).toEqual({
      input: { name: "France", isoCode: "FR", description: null },
    });
  });

  it("getRegionsByCountryId passes the country id and maps rows", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        regions: [
          {
            id: "9",
            name: "Bordeaux",
            description: null,
            country: {
              id: "1",
              name: "France",
              isoCode: "FR",
              description: null,
            },
          },
        ],
      })
    );

    const regions = await api.getRegionsByCountryId(1);
    expect(lastRequestBody().variables).toEqual({ countryId: "1" });
    expect(regions[0].regionName).toBe("Bordeaux");
    expect(regions[0].country?.countryName).toBe("France");
  });

  it("createRegion posts countryId, name, description", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createRegion: {
          id: "9",
          name: "Bordeaux",
          description: null,
          country: {
            id: "1",
            name: "France",
            isoCode: "FR",
            description: null,
          },
        },
      })
    );

    await api.createRegion({
      regionID: 0,
      regionName: "Bordeaux",
      countryID: 1,
      description: null,
      isActive: true,
      country: null,
      bottles: [],
    });
    expect(lastRequestBody().variables).toEqual({
      input: { countryId: "1", name: "Bordeaux", description: null },
    });
  });

  it("getTypes and createType round-trip", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        types: [{ id: "2", name: "Red", description: "Dry" }],
      })
    );
    const types = await api.getTypes();
    expect(types[0].typeName).toBe("Red");

    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createType: { id: "3", name: "White", description: null },
      })
    );
    const created = await api.createType({
      typeID: 0,
      typeName: "White",
      description: null,
      isActive: true,
      bottles: [],
    });
    expect(lastRequestBody().variables).toEqual({
      input: { name: "White", description: null },
    });
    expect(created.typeID).toBe(3);
  });

  it("getVintages maps year and isActive", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        vintages: [
          { id: "5", year: 2019, description: null, isActive: true },
          { id: "6", year: 2020, description: null, isActive: false },
        ],
      })
    );

    const vintages = await api.getVintages();
    expect(vintages.map((v) => v.year)).toEqual([2019, 2020]);
  });

  it("getActiveVintages filters inactive rows", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        vintages: [
          { id: "5", year: 2019, description: null, isActive: true },
          { id: "6", year: 2020, description: null, isActive: false },
        ],
      })
    );

    const vintages = await api.getActiveVintages();
    expect(vintages).toHaveLength(1);
    expect(vintages[0].year).toBe(2019);
  });

  it("createVintage posts year and description", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createVintage: {
          id: "7",
          year: 2021,
          description: null,
          isActive: true,
        },
      })
    );

    await api.createVintage({
      vintageID: 0,
      year: 2021,
      description: null,
      isActive: true,
      bottles: [],
    });
    expect(lastRequestBody().variables).toEqual({
      input: { year: 2021, description: null },
    });
  });

  it("getGrapeVarieties and getActiveGrapeVarieties filter correctly", async () => {
    const rows = [
      { id: "1", name: "Merlot", description: null, isActive: true },
      { id: "2", name: "Old", description: null, isActive: false },
    ];
    mockFetch.mockResolvedValueOnce(mockGraphQL({ grapeVarieties: rows }));
    expect(await api.getGrapeVarieties()).toHaveLength(2);

    mockFetch.mockResolvedValueOnce(mockGraphQL({ grapeVarieties: rows }));
    const active = await api.getActiveGrapeVarieties();
    expect(active).toHaveLength(1);
    expect(active[0].grapeVarietyName).toBe("Merlot");
  });

  it("updateGrapeVariety sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateGrapeVariety: {
          id: "1",
          name: "Merlot",
          description: "Updated",
          isActive: true,
        },
      })
    );

    await api.updateGrapeVariety(1, { description: "Updated" });
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { description: "Updated" },
    });
  });

  it("getWineFlavorProfiles maps rows and active filter works", async () => {
    const rows = [
      { id: "1", name: "Oaky", description: null, isActive: true },
      { id: "2", name: "Stale", description: null, isActive: false },
    ];
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ wineFlavorProfiles: rows })
    );
    const all = await api.getWineFlavorProfiles();
    expect(all).toHaveLength(2);

    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ wineFlavorProfiles: rows })
    );
    const active = await api.getActiveWineFlavorProfiles();
    expect(active).toHaveLength(1);
    expect(active[0].flavorProfileName).toBe("Oaky");
  });

  it("createWineFlavorProfile posts name and description", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createWineFlavorProfile: {
          id: "4",
          name: "Fruity",
          description: null,
          isActive: true,
        },
      })
    );

    await api.createWineFlavorProfile({
      flavorProfileID: 0,
      flavorProfileName: "Fruity",
      description: null,
      isActive: true,
    });
    expect(lastRequestBody().variables).toEqual({
      input: { name: "Fruity", description: null },
    });
  });
});

describe("api client: inventory reference data", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getNutrientTypes maps unit", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        nutrientTypes: [{ id: "1", name: "Calories", unit: "kcal" }],
      })
    );

    const types = await api.getNutrientTypes();
    expect(types[0].nutrientName).toBe("Calories");
    expect(types[0].unitOfMeasure).toBe("kcal");
  });

  it("createNutrientType posts name and unit", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createNutrientType: { id: "1", name: "Calories", unit: "kcal" },
      })
    );

    await api.createNutrientType({
      nutrientId: 0,
      nutrientName: "Calories",
      unitOfMeasure: "kcal",
    });
    expect(lastRequestBody().variables).toEqual({
      input: { name: "Calories", unit: "kcal" },
    });
  });

  it("updateNutrientType sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateNutrientType: { id: "1", name: "Calories", unit: "kJ" },
      })
    );

    await api.updateNutrientType(1, { unitOfMeasure: "kJ" });
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { unit: "kJ" },
    });
  });

  it("getFlavorProfiles and getActiveFlavorProfiles filter correctly", async () => {
    const rows = [
      { id: "1", name: "Spicy", isActive: true },
      { id: "2", name: "Bland", isActive: false },
    ];
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ flavorProfiles: rows })
    );
    expect(await api.getFlavorProfiles()).toHaveLength(2);

    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ flavorProfiles: rows })
    );
    const active = await api.getActiveFlavorProfiles();
    expect(active).toHaveLength(1);
    expect(active[0].flavorName).toBe("Spicy");
  });

  it("updateFlavorProfile sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateFlavorProfile: { id: "1", name: "Spicy", isActive: false },
      })
    );

    await api.updateFlavorProfile(1, { isActive: false });
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { isActive: false },
    });
  });
});

describe("api client: favorites and inventory adjustments", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("setItemFavorite posts itemId and isFavorite", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ setItemFavorite: { id: "1" } })
    );

    await api.setItemFavorite(1, true);
    expect(lastRequestBody().variables).toEqual({
      itemId: "1",
      isFavorite: true,
    });
  });

  it("setBottleFavorite posts bottleId and isFavorite", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ setBottleFavorite: { id: "2" } })
    );

    await api.setBottleFavorite(2, false);
    expect(lastRequestBody().variables).toEqual({
      bottleId: "2",
      isFavorite: false,
    });
  });

  it("setRecipeFavorite posts recipeId and isFavorite", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ setRecipeFavorite: true })
    );

    await api.setRecipeFavorite(3, true);
    expect(lastRequestBody().variables).toEqual({
      recipeId: "3",
      isFavorite: true,
    });
  });

  it("adjustItemQuantity posts quantity and optional purchase date", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ adjustUserItem: { id: "9" } })
    );

    await api.adjustItemQuantity(4, 2.5, "2026-09-05");
    expect(lastRequestBody().variables).toEqual({
      itemId: "4",
      quantity: 2.5,
      purchaseAt: "2026-09-05",
    });
  });

  it("adjustItemQuantity sends null purchaseAt when omitted", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ adjustUserItem: { id: "9" } })
    );

    await api.adjustItemQuantity(4, 1);
    expect(lastRequestBody().variables).toEqual({
      itemId: "4",
      quantity: 1,
      purchaseAt: null,
    });
  });
});

describe("api client: grocery list items", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  const gqlItem = {
    id: "10",
    manualItemName: "Paper towels",
    quantityNeeded: 2,
    unitOfMeasure: "ea",
    source: "manual",
    isChecked: false,
    item: null,
  };

  it("addGroceryListItem posts manual item fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ addGroceryItem: gqlItem })
    );

    const item = await api.addGroceryListItem(1, {
      itemID: null,
      itemName: null,
      manualItemName: "Paper towels",
      quantityNeeded: 2,
      unitOfMeasure: "ea",
      source: "manual",
      isChecked: false,
      createdBy: "",
      createDate: "",
      lastUpdatedBy: null,
      lastUpdatedDate: null,
    });
    expect(lastRequestBody().variables).toEqual({
      input: {
        groceryListId: "1",
        itemId: null,
        manualItemName: "Paper towels",
        quantity: 2,
        unit: "ea",
      },
    });
    expect(item.groceryListItemID).toBe(10);
  });

  it("toggleGroceryListItemChecked posts the item id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        toggleGroceryItemChecked: { ...gqlItem, isChecked: true },
      })
    );

    const item = await api.toggleGroceryListItemChecked(10);
    expect(lastRequestBody().variables).toEqual({
      groceryListItemId: "10",
    });
    expect(item.isChecked).toBe(true);
  });

  it("deleteGroceryListItem posts the item id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ deleteGroceryItem: true })
    );

    await api.deleteGroceryListItem(10);
    expect(lastRequestBody().variables).toEqual({
      groceryListItemId: "10",
    });
  });
});
