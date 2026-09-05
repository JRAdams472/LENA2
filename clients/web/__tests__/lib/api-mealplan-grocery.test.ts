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

describe("meal plan and grocery api client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("getMealPlan fetches the plan by id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        mealPlan: {
          id: "1",
          name: "Weekly",
          weekStartDate: "2026-09-01",
          isActive: true,
          slots: [],
        },
      })
    );

    const plan = await api.getMealPlan(1);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:5059/graphql",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Accept: "application/json" }),
      })
    );
    expect(plan.mealPlanID).toBe(1);
    expect(plan.planName).toBe("Weekly");
  });

  it("getMealPlanNutrition fetches nutrition", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        nutrition: [{ name: "Calories", unit: "kcal", amount: 120 }],
      })
    );

    const nutrition = await api.getMealPlanNutrition(1);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:5059/graphql",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Accept: "application/json" }),
      })
    );
    expect(nutrition.mealPlanId).toBe(1);
    expect(nutrition.dailyTotals).toHaveLength(1);
  });

  it("generateGroceryList requires a meal plan id", async () => {
    const error = await api.generateGroceryList().catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).message).toContain("requires a mealPlanId");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("generateGroceryList posts with a meal plan id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        generateGroceryList: {
          id: "2",
          generatedAt: "2026-09-01T00:00:00Z",
          items: [],
        },
      })
    );

    const list = await api.generateGroceryList(5);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:5059/graphql",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Accept: "application/json" }),
      })
    );
    expect(list.groceryListID).toBe(2);
  });

  it("getGroceryLists fetches all lists", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        groceryLists: {
          items: [
            {
              id: "1",
              generatedAt: "2026-09-01T00:00:00Z",
              items: [],
            },
          ],
          pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 },
        },
      })
    );

    const result = await api.getGroceryLists();

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:5059/graphql",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Accept: "application/json" }),
      })
    );
    expect(result).toHaveLength(1);
    expect(result[0].groceryListID).toBe(1);
  });
});
