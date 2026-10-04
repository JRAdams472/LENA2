import { api } from "@/lib/api";

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

const lastBody = () =>
  JSON.parse(mockFetch.mock.calls[mockFetch.mock.calls.length - 1][1].body);

describe("allergen api client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("getAllergens fetches the registry", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        allergens: [
          { id: "1", name: "Peanuts", description: null, isActive: true },
          { id: "2", name: "Gluten", description: "Celiac trigger", isActive: true },
        ],
      })
    );

    const out = await api.getAllergens();

    expect(lastBody().query).toContain("allergens");
    expect(out).toHaveLength(2);
    expect(out[0]).toEqual({
      allergenID: 1,
      name: "Peanuts",
      description: null,
      isActive: true,
    });
  });

  it("getMyAllergies maps member records", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        myAllergies: [
          {
            kind: "allergy",
            allergen: { id: "3", name: "Shellfish", description: null, isActive: true },
          },
          {
            kind: "dietary",
            allergen: { id: "7", name: "Pork", description: null, isActive: true },
          },
        ],
      })
    );

    const out = await api.getMyAllergies();

    expect(out).toEqual([
      {
        kind: "allergy",
        allergen: { allergenID: 3, name: "Shellfish", description: null, isActive: true },
      },
      {
        kind: "dietary",
        allergen: { allergenID: 7, name: "Pork", description: null, isActive: true },
      },
    ]);
  });

  it("setMyAllergy posts kind and on", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ setMyAllergy: true }));

    await api.setMyAllergy(5, "allergy", true);

    const body = lastBody();
    expect(body.query).toContain("setMyAllergy");
    expect(body.variables).toEqual({ allergenId: "5", kind: "allergy", on: true });
  });

  it("setMyAllergy clears with on=false", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ setMyAllergy: true }));

    await api.setMyAllergy(5, "dietary", false);

    expect(lastBody().variables).toEqual({ allergenId: "5", kind: "dietary", on: false });
  });

  it("createAllergen posts input", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        createAllergen: { id: "9", name: "Sesame", description: null, isActive: true },
      })
    );

    const out = await api.createAllergen({ name: "Sesame" });

    expect(lastBody().variables).toEqual({ input: { name: "Sesame", description: null } });
    expect(out.name).toBe("Sesame");
  });

  it("updateAllergen sends only provided fields", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        updateAllergen: { id: "9", name: "Sesame", description: null, isActive: false },
      })
    );

    const out = await api.updateAllergen(9, { isActive: false });

    expect(lastBody().variables).toEqual({ id: "9", input: { isActive: false } });
    expect(out.isActive).toBe(false);
  });

  it("setIngredientAllergen posts kind; null clears", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ setIngredientAllergen: true }));
    await api.setIngredientAllergen(10, 2, "may_contain");
    expect(lastBody().variables).toEqual({
      ingredientId: "10",
      allergenId: "2",
      kind: "may_contain",
    });

    mockFetch.mockResolvedValueOnce(mockGraphQL({ setIngredientAllergen: true }));
    await api.setIngredientAllergen(10, 2, null);
    expect(lastBody().variables).toEqual({
      ingredientId: "10",
      allergenId: "2",
      kind: null,
    });
  });

  it("setItemAllergen posts kind; null clears", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ setItemAllergen: true }));
    await api.setItemAllergen(11, 4, "contains");
    expect(lastBody().variables).toEqual({
      itemId: "11",
      allergenId: "4",
      kind: "contains",
    });

    mockFetch.mockResolvedValueOnce(mockGraphQL({ setItemAllergen: true }));
    await api.setItemAllergen(11, 4, null);
    expect(lastBody().variables).toEqual({ itemId: "11", allergenId: "4", kind: null });
  });

  it("getRecipe maps allergen flags and household warnings", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        recipe: {
          id: "5",
          name: "Pad Thai",
          description: null,
          servings: 4,
          prepTimeMinutes: 10,
          cookTimeMinutes: 15,
          items: [],
          steps: [],
          isFavorite: false,
          selectionCount: 0,
          personalSelectionCount: 0,
          myRating: null,
          averageRating: null,
          ratingCount: 0,
          categories: [],
          allergens: [
            {
              kind: "contains",
              allergen: { id: "1", name: "Peanuts", description: null, isActive: true },
            },
            {
              kind: "may_contain",
              allergen: { id: "3", name: "Shellfish", description: null, isActive: true },
            },
          ],
          allergyWarnings: [
            {
              memberKind: "allergy",
              entityKind: "contains",
              member: { id: "8", displayName: "Ada", firstName: null, lastName: null },
              allergen: { id: "1", name: "Peanuts", description: null, isActive: true },
            },
          ],
        },
      })
    );

    const recipe = await api.getRecipe(5);

    expect(recipe.allergens).toEqual([
      {
        kind: "contains",
        allergen: { allergenID: 1, name: "Peanuts", description: null, isActive: true },
      },
      {
        kind: "may_contain",
        allergen: { allergenID: 3, name: "Shellfish", description: null, isActive: true },
      },
    ]);
    expect(recipe.allergyWarnings).toEqual([
      {
        member: { userID: 8, displayName: "Ada", firstName: null, lastName: null },
        allergen: { allergenID: 1, name: "Peanuts", description: null, isActive: true },
        memberKind: "allergy",
        entityKind: "contains",
      },
    ]);
  });
});
