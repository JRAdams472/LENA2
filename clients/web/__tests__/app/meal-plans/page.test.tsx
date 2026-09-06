import "@testing-library/jest-dom";
import { Suspense } from "react";
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
    expect(screen.getByText("2024-01-01")).toBeInTheDocument();
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
    expect(screen.getByText(/2024-01-01/)).toBeInTheDocument();
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
});
