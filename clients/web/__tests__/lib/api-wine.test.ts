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

function lastRequestBody() {
  const [, init] = mockFetch.mock.calls[mockFetch.mock.calls.length - 1];
  return JSON.parse((init as RequestInit).body as string);
}

function gqlBottle(over: Record<string, unknown> = {}) {
  return {
    id: "1",
    typeId: "2",
    countryId: "3",
    regionId: "4",
    vineyard: "Napa",
    vintageYear: 2020,
    abv: 13.5,
    acidity: 5.5,
    tanninLevel: 3,
    body: 4,
    sweetness: 1,
    oakIntegration: true,
    bottleSize: "750ml",
    grapeVarieties: [
      {
        percentage: 85,
        grapeVariety: {
          id: "8",
          name: "Merlot",
          description: null,
          isActive: true,
        },
      },
    ],
    flavorProfiles: [
      {
        intensity: 2,
        flavorProfile: {
          id: "9",
          name: "Oaky",
          description: "oak",
          isActive: true,
        },
      },
    ],
    ...over,
  };
}

function bottlesPage(items: object[], totalCount = items.length) {
  return {
    bottles: {
      items,
      pageInfo: { pageNumber: 1, pageSize: 200, totalCount },
    },
  };
}

describe("api client: bottles", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getBottle maps nested grape varieties and flavor profiles", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ bottle: gqlBottle() }));

    const bottle = await api.getBottle(1);

    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(bottle.bottleID).toBe(1);
    expect(bottle.typeID).toBe(2);
    expect(bottle.countryID).toBe(3);
    expect(bottle.regionID).toBe(4);
    expect(bottle.oakIntegration).toBe(true);
    expect(bottle.bottleGrapeVarieties[0].grapeVarietyID).toBe(8);
    expect(bottle.bottleGrapeVarieties[0].percentage).toBe(85);
    expect(
      bottle.bottleGrapeVarieties[0].grapeVariety.grapeVarietyName
    ).toBe("Merlot");
    expect(bottle.bottleFlavorProfiles[0].flavorProfileName).toBe("Oaky");
    expect(bottle.quantity).toBe(0);
    expect(bottle.bottleNumber).toBeNull();
  });

  it("getBottle throws ApiError 404 when missing", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ bottle: null }));

    const error = await api.getBottle(42).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(404);
    expect((error as ApiError).message).toContain("Bottle 42 not found");
  });

  it("getBottle handles null collection fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        bottle: gqlBottle({ grapeVarieties: null, flavorProfiles: null }),
      })
    );

    const bottle = await api.getBottle(1);
    expect(bottle.bottleGrapeVarieties).toEqual([]);
    expect(bottle.bottleFlavorProfiles).toEqual([]);
  });

  it("createBottle posts the full input", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ createBottle: gqlBottle() })
    );

    const bottle = await api.createBottle({
      bottleID: 0,
      bottleNumber: null,
      typeID: 2,
      countryID: 3,
      regionID: 4,
      vintageYear: 2020,
      vineyard: "Napa",
      abv: 13.5,
      acidity: 5.5,
      tanninLevel: 3,
      body: 4,
      sweetness: 1,
      oakIntegration: true,
      bottleSize: "750ml",
      quantity: 0,
      purchaseDate: null,
      purchasePrice: null,
      storageTemp: null,
      location: null,
      notes: null,
      isFavorite: false,
      type: null,
      country: null,
      region: null,
      vintage: null,
      bottleGrapeVarieties: [],
      bottleFlavorProfiles: [],
    });

    expect(lastRequestBody().variables).toEqual({
      input: {
        typeId: "2",
        countryId: "3",
        regionId: "4",
        vintageYear: 2020,
        vineyard: "Napa",
        abv: 13.5,
        acidity: 5.5,
        tanninLevel: 3,
        body: 4,
        sweetness: 1,
        oakIntegration: true,
        bottleSize: "750ml",
      },
    });
    expect(bottle.bottleID).toBe(1);
  });

  it("updateBottle sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ updateBottle: gqlBottle({ vineyard: "Sonoma" }) })
    );

    const bottle = await api.updateBottle(1, {
      vineyard: "Sonoma",
      typeID: 5,
      countryID: 6,
      regionID: 7,
      vintageYear: 2021,
      abv: 14,
      acidity: 5,
      tanninLevel: 2,
      body: 3,
      sweetness: 2,
      oakIntegration: false,
      bottleSize: "1.5L",
    });

    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: {
        typeId: "5",
        countryId: "6",
        regionId: "7",
        vintageYear: 2021,
        vineyard: "Sonoma",
        abv: 14,
        acidity: 5,
        tanninLevel: 2,
        body: 3,
        sweetness: 2,
        oakIntegration: false,
        bottleSize: "1.5L",
      },
    });
    expect(bottle.vineyard).toBe("Sonoma");
  });

  it("deleteBottle posts the id and resolves null", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ deleteBottle: true }));

    const result = await api.deleteBottle(1);
    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(result).toBeNull();
  });

  it("getBottlesPaged passes filter variables in a single query", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(bottlesPage([gqlBottle()], 57))
    );

    const result = await api.getBottlesPaged(2, 25, {
      search: "napa",
      countryId: 3,
      regionId: 4,
      typeId: 2,
      vintageYear: 2020,
      favoritesOnly: true,
    });

    const body = lastRequestBody();
    expect(body.query).toContain(
      "bottles(page: $page, pageSize: $pageSize, search: $search, countryId: $countryId, regionId: $regionId, typeId: $typeId, vintageYear: $vintageYear, favoritesOnly: $favoritesOnly)"
    );
    expect(body.variables).toEqual({
      page: 2,
      pageSize: 25,
      search: "napa",
      countryId: "3",
      regionId: "4",
      typeId: "2",
      vintageYear: 2020,
      favoritesOnly: true,
    });
    expect(result.totalCount).toBe(57);
    expect(result.items[0].bottleID).toBe(1);
  });

  it("getBottlesPaged sends nulls for omitted filters", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL(bottlesPage([gqlBottle()])));

    await api.getBottlesPaged(1, 25);

    expect(lastRequestBody().variables).toEqual({
      page: 1,
      pageSize: 25,
      search: null,
      countryId: null,
      regionId: null,
      typeId: null,
      vintageYear: null,
      favoritesOnly: null,
    });
  });

  it("getBottlesByCountryId issues one filtered query", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(bottlesPage([gqlBottle()]))
    );

    const result = await api.getBottlesByCountryId(3);
    expect(lastRequestBody().variables.countryId).toBe("3");
    expect(result.items).toHaveLength(1);
  });

  it("getBottlesByRegionId issues one filtered query", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(bottlesPage([gqlBottle()]))
    );

    const result = await api.getBottlesByRegionId(4);
    expect(lastRequestBody().variables.regionId).toBe("4");
    expect(result.items).toHaveLength(1);
  });

  it("getBottlesByTypeId issues one filtered query", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(bottlesPage([gqlBottle()]))
    );

    const result = await api.getBottlesByTypeId(2);
    expect(lastRequestBody().variables.typeId).toBe("2");
    expect(result.items).toHaveLength(1);
  });

  it("getBottlesByVintageYear issues one filtered query", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(bottlesPage([gqlBottle()]))
    );

    const result = await api.getBottlesByVintageYear(2020);
    expect(lastRequestBody().variables.vintageYear).toBe(2020);
    expect(result.items[0].vintageYear).toBe(2020);
  });

  it("searchBottles issues a ranked server-side query", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(
        bottlesPage([gqlBottle({ id: "1", vineyard: "Napa Valley" })])
      )
    );

    const bottles = await api.searchBottles("napa");
    const body = lastRequestBody();
    expect(body.query).toContain("search: $search");
    expect(body.variables.search).toBe("napa");
    expect(bottles).toHaveLength(1);
    expect(bottles[0].vineyard).toBe("Napa Valley");
  });

  it("getBottleCount returns pageInfo totalCount", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        bottles: {
          pageInfo: { pageNumber: 1, pageSize: 1, totalCount: 42 },
        },
      })
    );

    expect(await api.getBottleCount()).toBe(42);
  });

  it("getFavoriteBottles issues one favorites-only query", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL(bottlesPage([gqlBottle()]))
    );

    const result = await api.getFavoriteBottles();
    expect(lastRequestBody().variables.favoritesOnly).toBe(true);
    expect(result.items).toHaveLength(1);
  });
});

describe("api client: wine reference CRUD", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("getCountriesPaged slices the full country list", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        countries: [
          { id: "1", name: "France", isoCode: "FR", description: null },
          { id: "2", name: "Italy", isoCode: null, description: null },
          { id: "3", name: "Spain", isoCode: "ES", description: null },
        ],
      })
    );

    const result = await api.getCountriesPaged(2, 2);
    expect(result.items).toHaveLength(1);
    expect(result.items[0].countryName).toBe("Spain");
    expect(result.totalPages).toBe(2);
  });

  it("getActiveCountries returns all countries", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        countries: [
          { id: "1", name: "France", isoCode: "FR", description: null },
        ],
      })
    );

    const countries = await api.getActiveCountries();
    expect(countries).toHaveLength(1);
    expect(countries[0].isActive).toBe(true);
  });

  it("updateCountry sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateCountry: {
          id: "1",
          name: "France",
          isoCode: "FR",
          description: "d",
        },
      })
    );

    await api.updateCountry(1, { description: "d" });
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { description: "d" },
    });
  });

  it("updateCountry sends name and isoCode when provided", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateCountry: {
          id: "1",
          name: "France",
          isoCode: "FR",
          description: null,
        },
      })
    );

    await api.updateCountry(1, {
      countryName: "France",
      isoCode: "FR",
      isActive: false,
    });
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { name: "France", isoCode: "FR", isActive: false },
    });
  });

  it("deleteCountry posts the id", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ deleteCountry: true }));

    const result = await api.deleteCountry(1);
    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(result).toBeNull();
  });

  it("getRegions fetches regions for each country", async () => {
    const france = {
      id: "1",
      name: "France",
      isoCode: "FR",
      description: null,
    };
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL({
          countries: [
            france,
            { id: "2", name: "Italy", isoCode: "IT", description: null },
          ],
        })
      )
      .mockResolvedValueOnce(
        mockGraphQL({
          regions: [
            {
              id: "9",
              name: "Bordeaux",
              description: null,
              country: france,
            },
          ],
        })
      )
      .mockResolvedValueOnce(mockGraphQL({ regions: [] }));

    const regions = await api.getRegions();

    expect(mockFetch).toHaveBeenCalledTimes(3);
    expect(regions).toHaveLength(1);
    expect(regions[0].regionName).toBe("Bordeaux");
    expect(regions[0].countryID).toBe(1);
  });

  it("getRegionsPaged slices regions across countries", async () => {
    const france = {
      id: "1",
      name: "France",
      isoCode: "FR",
      description: null,
    };
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ countries: [france] }))
      .mockResolvedValueOnce(
        mockGraphQL({
          regions: [
            { id: "9", name: "Bordeaux", description: null, country: france },
            { id: "10", name: "Burgundy", description: null, country: france },
          ],
        })
      );

    const result = await api.getRegionsPaged(2, 1);
    expect(result.items).toHaveLength(1);
    expect(result.items[0].regionName).toBe("Burgundy");
    expect(result.totalCount).toBe(2);
  });

  it("getRegions maps a null country to null", async () => {
    mockFetch
      .mockResolvedValueOnce(
        mockGraphQL({
          countries: [
            { id: "1", name: "France", isoCode: "FR", description: null },
          ],
        })
      )
      .mockResolvedValueOnce(
        mockGraphQL({
          regions: [
            { id: "9", name: "Nowhere", description: null, country: null },
          ],
        })
      );

    const regions = await api.getRegions();
    expect(regions[0].country).toBeNull();
    expect(regions[0].countryID).toBe(0);
  });

  it("updateRegion sends only provided fields", async () => {
    const france = {
      id: "1",
      name: "France",
      isoCode: "FR",
      description: null,
    };
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateRegion: {
          id: "9",
          name: "Bordeaux",
          description: "d",
          country: france,
        },
      })
    );

    await api.updateRegion(9, {
      countryID: 1,
      regionName: "Bordeaux",
      isActive: true,
    });
    expect(lastRequestBody().variables).toEqual({
      id: "9",
      input: { countryId: "1", name: "Bordeaux", isActive: true },
    });
  });

  it("deleteRegion posts the id", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ deleteRegion: true }));

    const result = await api.deleteRegion(9);
    expect(lastRequestBody().variables).toEqual({ id: "9" });
    expect(result).toBeNull();
  });

  it("getTypesPaged slices the type list", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        types: [
          { id: "1", name: "Red", description: null },
          { id: "2", name: "White", description: null },
        ],
      })
    );

    const result = await api.getTypesPaged(1, 1);
    expect(result.items).toHaveLength(1);
    expect(result.items[0].typeName).toBe("Red");
    expect(result.totalPages).toBe(2);
  });

  it("updateType sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateType: { id: "1", name: "Rosé", description: null },
      })
    );

    const type = await api.updateType(1, { typeName: "Rosé" });
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { name: "Rosé" },
    });
    expect(type.typeName).toBe("Rosé");
  });

  it("deleteType posts the id", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ deleteType: true }));

    const result = await api.deleteType(1);
    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(result).toBeNull();
  });

  it("getVintagesPaged slices the vintage list", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        vintages: [
          { id: "1", year: 2019, description: null, isActive: true },
          { id: "2", year: 2020, description: null, isActive: false },
        ],
      })
    );

    const result = await api.getVintagesPaged(1, 1);
    expect(result.items).toHaveLength(1);
    expect(result.items[0].year).toBe(2019);
    expect(result.totalPages).toBe(2);
  });

  it("updateVintage sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateVintage: {
          id: "1",
          year: 2022,
          description: null,
          isActive: true,
        },
      })
    );

    const vintage = await api.updateVintage(1, { year: 2022 });
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { year: 2022 },
    });
    expect(vintage.year).toBe(2022);
  });

  it("deleteVintage posts the id", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ deleteVintage: true }));

    const result = await api.deleteVintage(1);
    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(result).toBeNull();
  });

  it("createGrapeVariety posts name and description", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createGrapeVariety: {
          id: "2",
          name: "Syrah",
          description: null,
          isActive: true,
        },
      })
    );

    const gv = await api.createGrapeVariety({
      grapeVarietyID: 0,
      grapeVarietyName: "Syrah",
      description: null,
      isActive: true,
      bottleGrapeVarieties: [],
    });
    expect(lastRequestBody().variables).toEqual({
      input: { name: "Syrah", description: null },
    });
    expect(gv.grapeVarietyID).toBe(2);
  });

  it("deleteGrapeVariety posts the id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ deleteGrapeVariety: true })
    );

    const result = await api.deleteGrapeVariety(2);
    expect(lastRequestBody().variables).toEqual({ id: "2" });
    expect(result).toBeNull();
  });

  it("updateWineFlavorProfile sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateWineFlavorProfile: {
          id: "1",
          name: "Oaky",
          description: "d",
          isActive: false,
        },
      })
    );

    const profile = await api.updateWineFlavorProfile(1, {
      isActive: false,
      description: "d",
    });
    expect(lastRequestBody().variables).toEqual({
      id: "1",
      input: { description: "d", isActive: false },
    });
    expect(profile.isActive).toBe(false);
  });

  it("deleteWineFlavorProfile posts the id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ deleteWineFlavorProfile: true })
    );

    const result = await api.deleteWineFlavorProfile(1);
    expect(lastRequestBody().variables).toEqual({ id: "1" });
    expect(result).toBeNull();
  });
});
