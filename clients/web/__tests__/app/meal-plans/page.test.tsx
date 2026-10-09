import "@testing-library/jest-dom";
import { Suspense } from "react";
import userEvent from "@testing-library/user-event";
import { act, render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import MealPlansPage from "@/app/meal-plans/page";
import MealPlanDetailPage from "@/app/meal-plans/[id]/page";

const mockFetch = global.fetch as jest.Mock;
const mockPush = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: mockPush })),
  useParams: jest.fn(() => ({ id: "1" })),
  usePathname: jest.fn(() => "/meal-plans"),
}));

function gql(data: object) {
  return {
    ok: true,
    status: 200,
    headers: { get: () => "application/json" },
    json: async () => ({ data }),
  };
}

function getBodies() {
  return mockFetch.mock.calls.map((c) =>
    JSON.parse((c[1] as RequestInit).body as string)
  );
}

function renderPage(element: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>{element}</QueryClientProvider>
  );
}

// The detail page resolves `params` via React's `use()`, which suspends —
// the render must be awaited inside act() with a Suspense boundary.
async function renderDetailPage(element: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  let result: ReturnType<typeof render> | undefined;
  await act(async () => {
    result = render(
      <QueryClientProvider client={queryClient}>
        <Suspense fallback={null}>{element}</Suspense>
      </QueryClientProvider>
    );
  });
  return result!;
}

const plan = {
  id: "1",
  name: "Plan 1",
  weekStartDate: "2024-01-01T00:00:00Z",
  isActive: true,
  slots: [],
};

beforeEach(() => {
  mockFetch.mockReset();
  mockPush.mockReset();
  jest.spyOn(window, "confirm").mockReturnValue(true);
});

afterEach(() => {
  jest.restoreAllMocks();
});

describe("meal plans page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createMealPlan")) {
        return Promise.resolve(gql({ createMealPlan: { ...plan, id: "2", name: "Plan 2" } }));
      }
      if (body.query.includes("updateMealPlan")) {
        return Promise.resolve(gql({ updateMealPlan: { ...plan, name: "Plan 1 Updated" } }));
      }
      if (body.query.includes("deleteMealPlan")) {
        return Promise.resolve(gql({ deleteMealPlan: true }));
      }
      if (body.query.includes("mealPlan(")) {
        return Promise.resolve(gql({ mealPlan: plan }));
      }
      return Promise.resolve(
        gql({
          mealPlans: {
            items: [plan],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      );
    });
  });

  it("lists meal plans", async () => {
    renderPage(<MealPlansPage />);
    await waitFor(() => expect(screen.getByText("Plan 1")).toBeInTheDocument());
    expect(
      screen.getByText(new Date(2024, 0, 1).toLocaleDateString())
    ).toBeInTheDocument();
  });

  it("creates a meal plan", async () => {
    renderPage(<MealPlansPage />);
    await waitFor(() => screen.getByText("Plan 1"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Plan Name"), { target: { value: "Plan 2" } });
    fireEvent.change(screen.getByLabelText("Week Start Date"), { target: { value: "2024-01-08" } });
    fireEvent.change(screen.getByLabelText("Week Start Day (0=Sun)"), { target: { value: "1" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createMealPlan"))).toBe(true);
    });
  });

  it("edits a meal plan", async () => {
    renderPage(<MealPlansPage />);
    await waitFor(() => screen.getByText("Plan 1"));
    const row = screen.getByText("Plan 1").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Plan Name"), { target: { value: "Plan 1 Updated" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateMealPlan") && b.variables.id === "1")).toBe(true);
    });
  });

  it("deletes a meal plan", async () => {
    renderPage(<MealPlansPage />);
    await waitFor(() => screen.getByText("Plan 1"));
    const row = screen.getByText("Plan 1").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteMealPlan") && b.variables.id === "1")).toBe(true);
    });
  });
});

describe("meal plan detail page", () => {
  const item = {
    id: "1",
    name: "Milk",
    upc12: null,
    upc14: null,
    unit: "gallon",
    brand: { id: "2", name: "DairyCo" },
    category: { id: "3", name: "Dairy", description: null },
    nutrients: [],
    flavors: [],
  };

  const userItem = {
    id: "10",
    currentQty: 5,
    minQty: null,
    purchaseAt: null,
    expiresAt: null,
    notes: null,
    isFavorite: false,
    item: { id: "1" },
  };

  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("generateGroceryList")) {
        return Promise.resolve(
          gql({ generateGroceryList: { id: "2", generatedAt: "2024-01-02T00:00:00Z", items: [] } })
        );
      }
      if (body.query.includes("mealPlan(")) {
        return Promise.resolve(gql({ mealPlan: plan }));
      }
      if (body.query.includes("recipes")) {
        return Promise.resolve(gql({ recipes: { items: [], pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 } } }));
      }
      if (body.query.includes("nutrition(")) {
        return Promise.resolve(gql({ nutrition: [] }));
      }
      if (body.query.includes("userItems")) {
        return Promise.resolve(
          gql({
            userItems: {
              items: [userItem],
              pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 },
            },
          })
        );
      }
      return Promise.resolve(
        gql({
          items: {
            items: [item],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 },
          },
        })
      );
    });
  });

  it("renders the meal plan", async () => {
    await renderDetailPage(<MealPlanDetailPage params={Promise.resolve({ id: "1" })} />);
    await waitFor(() => expect(screen.getByText("Plan 1")).toBeInTheDocument());
    expect(screen.getByText(/Week starting/).textContent).toContain(
      new Date(2024, 0, 1).toLocaleDateString()
    );
    expect(screen.getByText("Weekly Grid")).toBeInTheDocument();
  });

  it("generates a grocery list", async () => {
    await renderDetailPage(<MealPlanDetailPage params={Promise.resolve({ id: "1" })} />);
    await waitFor(() => screen.getByText("Plan 1"));
    fireEvent.click(screen.getByRole("button", { name: /generate grocery list/i }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("generateGroceryList"))).toBe(true);
      expect(mockPush).toHaveBeenCalledWith("/grocery-lists/2");
    });
  });

  describe("slots, suggestions, and nutrition", () => {
    const recipe = {
      id: "5",
      name: "Chili",
      description: "Hearty",
      servings: 4,
      prepTimeMinutes: 10,
      cookTimeMinutes: 30,
      isFavorite: false,
      items: [],
      steps: [],
      categories: [],
      allergens: [],
      allergyWarnings: [],
    };

    const sourCream = {
      id: "2",
      name: "Sour Cream",
      upc12: null,
      upc14: null,
      unit: "cup",
      brand: null,
      category: null,
      nutrients: [],
      flavors: [],
    };

    const recipeWithOptional = {
      ...recipe,
      items: [
        {
          id: "40",
          quantity: 0.5,
          unit: "cup",
          isOptional: true,
          ingredient: null,
          item: sourCream,
          section: null,
          notes: null,
        },
      ],
    };

    const slot = {
      id: "20",
      dayOfWeek: 1,
      mealType: "dinner",
      recipe,
      servings: 4,
      replacementNote: "use canned beans",
      items: [
        {
          id: "30",
          item,
          ingredient: null,
          quantity: 1,
          unit: "cup",
          isFromRecipe: false,
        },
      ],
    };

    const plannedPlan = { ...plan, slots: [slot] };

    function mockDetail(opts: {
      thePlan?: object;
      aiAvailable?: boolean;
      suggestions?: object[] | null;
    } = {}) {
      mockFetch.mockImplementation((_, init) => {
        const body = JSON.parse((init as RequestInit).body as string);
        // Order matters: addMealSlotItem/removeMealSlotItem contain the
        // addMealSlot/removeMealSlot prefixes.
        if (body.query.includes("addMealSlotItem")) {
          return Promise.resolve(
            gql({
              addMealSlotItem: {
                id: "31",
                quantity: 1,
                unit: "cup",
                isFromRecipe: false,
                ingredient: null,
                item: null,
              },
            })
          );
        }
        if (body.query.includes("removeMealSlotItem")) {
          return Promise.resolve(gql({ removeMealSlotItem: true }));
        }
        if (body.query.includes("addMealSlot")) {
          return Promise.resolve(
            gql({
              addMealSlot: {
                id: "21",
                dayOfWeek: body.variables.input.dayOfWeek,
                mealType: body.variables.input.mealType,
                servings: body.variables.input.servings,
                replacementNote: body.variables.input.replacementNote,
                recipe: null,
                items: [],
              },
            })
          );
        }
        if (body.query.includes("removeMealSlot")) {
          return Promise.resolve(gql({ removeMealSlot: true }));
        }
        if (body.query.includes("suggestMeals")) {
          return Promise.resolve(gql({ suggestMeals: opts.suggestions ?? [] }));
        }
        if (body.query.includes("aiAvailable")) {
          return Promise.resolve(gql({ aiAvailable: opts.aiAvailable ?? false }));
        }
        if (body.query.includes("recipeCategoryGroups")) {
          return Promise.resolve(
            gql({
              recipeCategoryGroups: [
                {
                  id: "1",
                  name: "Course",
                  exclusive: true,
                  displayOrder: 0,
                  categories: [
                    {
                      id: "10",
                      name: "Main",
                      group: { id: "1", name: "Course", exclusive: true, displayOrder: 0 },
                    },
                  ],
                },
              ],
            })
          );
        }
        if (body.query.includes("frequentBrands") || body.query.includes("searchBrands")) {
          return Promise.resolve(gql({ frequentBrands: [], searchBrands: [] }));
        }
        if (body.query.includes("recordSelection") || body.query.includes("recordSearch")) {
          return Promise.resolve(gql({ recordSelection: true, recordSearch: true }));
        }
        // The recipe detail query contains `items(view: $view)` and
        // `steps(view: $view)` — check it before the bare `items(` branch.
        if (body.query.includes("recipe(")) {
          return Promise.resolve(gql({ recipe: recipeWithOptional }));
        }
        if (body.query.includes("items(")) {
          return Promise.resolve(gql({ items: { items: [item] } }));
        }
        if (body.query.includes("item(")) {
          return Promise.resolve(gql({ item }));
        }
        if (body.query.includes("recipes")) {
          return Promise.resolve(
            gql({ recipes: { items: [recipe], pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 } } })
          );
        }
        if (body.query.includes("nutrition(")) {
          return Promise.resolve(
            gql({
              nutrition: {
                entries: [{ name: "Calories", unit: "kcal", amount: 600 }],
                warnings: ["High sodium"],
              },
            })
          );
        }
        if (body.query.includes("userItems")) {
          return Promise.resolve(
            gql({ userItems: { items: [], pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 } } })
          );
        }
        if (body.query.includes("generateGroceryList")) {
          return Promise.resolve(
            gql({ generateGroceryList: { id: "2", generatedAt: "2024-01-02T00:00:00Z", items: [] } })
          );
        }
        return Promise.resolve(gql({ mealPlan: opts.thePlan ?? plan }));
      });
    }

    it("renders a planned slot with chips and the nutrition panel", async () => {
      mockDetail({ thePlan: plannedPlan });
      await renderDetailPage(<MealPlanDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => expect(screen.getByText("Chili")).toBeInTheDocument());
      expect(screen.getByText("use canned beans")).toBeInTheDocument();
      await waitFor(() => expect(screen.getByText("Nutrition")).toBeInTheDocument());
      expect(screen.getByText("High sodium")).toBeInTheDocument();
      expect(screen.getByText(/Calories: 600\.00 kcal/)).toBeInTheDocument();
    });

    it("opens a filled slot and saves it via remove+add", async () => {
      mockDetail({ thePlan: plannedPlan });
      await renderDetailPage(<MealPlanDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Chili"));

      fireEvent.click(screen.getByText("Chili"));
      await screen.findByText("Mon - Dinner");
      // Optional-ingredients picker renders once the recipe detail loads —
      // the MUI Select renders its label text in two nodes.
      await screen.findAllByText("Include Optional Ingredients");

      fireEvent.change(screen.getByLabelText("Servings"), { target: { value: "6" } });
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
      await waitFor(() => {
        const bodies = getBodies();
        expect(bodies.some((b) => b.query.includes("removeMealSlot"))).toBe(true);
        expect(bodies.some((b) => b.query.includes("addMealSlot"))).toBe(true);
      });
    });

    it("adds a slot from an empty cell", async () => {
      mockDetail();
      await renderDetailPage(<MealPlanDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Plan 1"));

      fireEvent.click(screen.getAllByText("Nothing planned — tap to add")[0]);
      await screen.findByText("Sun - Breakfast");
      expect(screen.queryByRole("button", { name: "Remove Slot" })).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Save" }));
      await waitFor(() =>
        expect(getBodies().some((b) => b.query.includes("addMealSlot"))).toBe(true)
      );
    });

    it("removes a slot", async () => {
      mockDetail({ thePlan: plannedPlan });
      await renderDetailPage(<MealPlanDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Chili"));

      fireEvent.click(screen.getByText("Chili"));
      await screen.findByText("Mon - Dinner");
      fireEvent.click(screen.getByRole("button", { name: "Remove Slot" }));
      await waitFor(() =>
        expect(getBodies().some((b) => b.query.includes("removeMealSlot"))).toBe(true)
      );
    });

    it("suggests meals and adds a suggestion to the plan", async () => {
      mockDetail({
        aiAvailable: true,
        suggestions: [
          {
            recipe,
            dayOfWeek: 2,
            mealType: "lunch",
            reason: "Light lunch",
            usesExpiringItems: ["Spinach"],
          },
        ],
      });
      await renderDetailPage(<MealPlanDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Plan 1"));

      fireEvent.click(screen.getByRole("button", { name: "Suggest Meals" }));
      await screen.findByText("Suggested for your week");
      expect(screen.getByText("Light lunch")).toBeInTheDocument();
      expect(screen.getByText("uses soon: Spinach")).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Add to plan" }));
      await waitFor(() => {
        const body = getBodies().find((b) => b.query.includes("addMealSlot"));
        expect(body?.variables.input.recipeId).toBe("5");
      });
      // The taken suggestion is filtered out of the list.
      await waitFor(() => expect(screen.queryByText("Light lunch")).not.toBeInTheDocument());
    });

    it("hides Suggest Meals when AI is unavailable", async () => {
      mockDetail({ aiAvailable: false });
      await renderDetailPage(<MealPlanDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Plan 1"));
      expect(screen.queryByRole("button", { name: "Suggest Meals" })).not.toBeInTheDocument();
    });

    it("adds an ad-hoc item to a slot", async () => {
      mockDetail();
      await renderDetailPage(<MealPlanDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Plan 1"));

      fireEvent.click(screen.getAllByText("Nothing planned — tap to add")[0]);
      await screen.findByText("Sun - Breakfast");

      // MUI Autocomplete needs the full keystroke sequence — bare
      // fireEvent.change doesn't reach onInputChange here.
      const itemInput = screen.getByLabelText("Item");
      await userEvent.type(itemInput, "mil");
      // The 300ms debounce must elapse before the search fires.
      await waitFor(
        () => expect(getBodies().some((b) => b.query.includes("items("))).toBe(true),
        { timeout: 3000 }
      );
      const option = await screen.findByRole("option", { name: /Milk/ });
      await userEvent.click(option);

      fireEvent.change(screen.getByLabelText("Qty"), { target: { value: "2" } });
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
      // The unit auto-fills from the picked item (gallon).
      await waitFor(() => expect(screen.getByText(/2 gallon/)).toBeInTheDocument());

      fireEvent.click(screen.getByRole("button", { name: "Save" }));
      await waitFor(() => {
        const bodies = getBodies();
        expect(bodies.some((b) => b.query.includes("addMealSlot"))).toBe(true);
        expect(bodies.some((b) => b.query.includes("addMealSlotItem"))).toBe(true);
      });
    });
  });
});
