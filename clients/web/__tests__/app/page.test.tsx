import "@testing-library/jest-dom";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Dashboard from "@/app/page";

const mockFetch = global.fetch as jest.Mock;

function gql(data: object) {
  return {
    ok: true,
    status: 200,
    headers: { get: () => "application/json" },
    json: async () => ({ data }),
  };
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <Dashboard />
    </QueryClientProvider>
  );
}

const todayIso = new Date().toISOString().split("T")[0] + "T00:00:00Z";

const plan = {
  id: "1",
  name: "This Week",
  weekStartDate: todayIso,
  isActive: true,
  slots: [
    {
      id: "1",
      dayOfWeek: new Date().getDay(),
      mealType: "breakfast",
      servings: 1,
      replacementNote: null,
      recipe: { id: "1", name: "Pancakes", description: null, servings: 2, prepTimeMinutes: 5, cookTimeMinutes: 10, isFavorite: false, items: [], steps: [] },
      items: [],
    },
  ],
};

describe("dashboard page", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("recipes")) {
        return Promise.resolve(
          gql({ recipes: { items: [plan.slots[0].recipe], pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 } } })
        );
      }
      if (body.query.includes("mealPlan(")) {
        return Promise.resolve(gql({ mealPlan: plan }));
      }
      return Promise.resolve(
        gql({ mealPlans: { items: [plan], pageInfo: { pageNumber: 1, pageSize: 1000, totalCount: 1 } } })
      );
    });
  });

  it("renders the dashboard for the current week", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Dashboard")).toBeInTheDocument());
    expect(screen.getByText("Breakfast")).toBeInTheDocument();
    expect(screen.getByText("Pancakes")).toBeInTheDocument();
  });
});
