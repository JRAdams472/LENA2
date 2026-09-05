import { api, ApiError, setAuthTokenGetter } from "@/lib/api";

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

describe("api client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
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
});
