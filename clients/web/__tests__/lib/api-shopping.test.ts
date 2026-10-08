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

function mockGraphQLError(message: string) {
  return {
    ok: true,
    status: 200,
    headers: {
      get: (name: string) =>
        name === "content-type" ? "application/json" : null,
    },
    json: async () => ({ errors: [{ message }] }),
  };
}

function requestBody(): { query: string; variables: Record<string, unknown> } {
  return JSON.parse(mockFetch.mock.calls[0][1].body as string);
}

describe("shopping link api client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("getShopperProviders queries the capability list", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ shopperProviders: ["INSTACART"] })
    );

    const providers = await api.getShopperProviders();

    expect(requestBody().query).toContain("shopperProviders");
    expect(providers).toEqual(["INSTACART"]);
  });

  it("getShopperProviders returns the empty list untouched", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ shopperProviders: [] })
    );

    expect(await api.getShopperProviders()).toEqual([]);
  });

  it("createShoppingLink posts the mutation with provider and includeChecked", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createShoppingLink: {
          provider: "INSTACART",
          url: "https://instacart.example.com/list/abc123",
        },
      })
    );

    const link = await api.createShoppingLink(7, "INSTACART", true);

    const body = requestBody();
    expect(body.query).toContain("createShoppingLink");
    expect(body.variables).toEqual({
      groceryListId: "7",
      provider: "INSTACART",
      includeChecked: true,
    });
    expect(link).toEqual({
      provider: "INSTACART",
      url: "https://instacart.example.com/list/abc123",
    });
  });

  it("createShoppingLink defaults to INSTACART excluding checked items", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createShoppingLink: {
          provider: "INSTACART",
          url: "https://instacart.example.com/list/def456",
        },
      })
    );

    await api.createShoppingLink(3);

    expect(requestBody().variables).toEqual({
      groceryListId: "3",
      provider: "INSTACART",
      includeChecked: false,
    });
  });

  it("createShoppingLink surfaces GraphQL errors", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQLError("no shopping provider configured")
    );

    await expect(api.createShoppingLink(1)).rejects.toBeInstanceOf(ApiError);
  });
});
