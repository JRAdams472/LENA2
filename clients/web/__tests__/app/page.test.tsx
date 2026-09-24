import "@testing-library/jest-dom";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Dashboard from "@/app/page";

// jest.mock specifiers are not rewritten by the SWC path transform, so
// the "@/..." alias cannot be used here — mock the resolved path instead.
jest.mock("../../app/auth/useMe");
import { useMe } from "@/app/auth/useMe";

const mockedUseMe = useMe as jest.Mock;
const mockFetch = global.fetch as jest.Mock;

const me = {
  userID: 7,
  email: "me@example.com",
  displayName: "Me",
  firstName: null,
  lastName: null,
  backupEmail: null,
  role: "member" as const,
  isActive: true,
  isProtected: false,
  lastLoginAt: null,
  externalSubject: null,
  provider: null,
  isSearchable: true,
  household: null,
};

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
    mockedUseMe.mockReturnValue({
      me,
      isAdmin: false,
      isLoading: false,
      error: null,
      refetch: jest.fn(),
    });
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("householdInvites")) {
        return Promise.resolve(gql({ householdInvites: [] }));
      }
      if (body.query.includes("recommendedRecipes")) {
        return Promise.resolve(gql({ recommendedRecipes: [] }));
      }
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

  it("shows the empty suggestions state", async () => {
    renderPage();
    await waitFor(() =>
      expect(
        screen.getByText(/No suggestions yet/)
      ).toBeInTheDocument()
    );
  });

  it("renders suggested recipes with reason labels", async () => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("recommendedRecipes")) {
        return Promise.resolve(
          gql({
            recommendedRecipes: [
              {
                recipe: { ...plan.slots[0].recipe, id: "9", name: "Curry" },
                reason: "ingredient_overlap",
                score: 0.8,
              },
              {
                recipe: { ...plan.slots[0].recipe, id: "10", name: "Stew" },
                reason: "rating_recency",
                score: 0.6,
              },
            ],
          })
        );
      }
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

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("Curry")).toBeInTheDocument()
    );
    expect(screen.getByText("Stew")).toBeInTheDocument();
    expect(screen.getByText(/Similar to your menu/)).toBeInTheDocument();
    expect(screen.getByText(/Due for a revisit/)).toBeInTheDocument();
  });

  it("shows pending household invites and accepts one", async () => {
    const invite = {
      id: "44",
      status: "PENDING",
      createdAt: "2026-09-20T10:00:00Z",
      fromUser: { id: "9", displayName: "Inviter", firstName: null, lastName: null },
      toUser: { id: "7", displayName: "Me", firstName: null, lastName: null },
    };
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("acceptHouseholdInvite")) {
        return Promise.resolve(
          gql({
            acceptHouseholdInvite: {
              id: "42",
              members: [],
              createdAt: "2026-01-01T00:00:00Z",
            },
          })
        );
      }
      if (body.query.includes("householdInvites")) {
        return Promise.resolve(gql({ householdInvites: [invite] }));
      }
      if (body.query.includes("recommendedRecipes")) {
        return Promise.resolve(gql({ recommendedRecipes: [] }));
      }
      if (body.query.includes("recipes")) {
        return Promise.resolve(
          gql({ recipes: { items: [], pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 } } })
        );
      }
      if (body.query.includes("mealPlan(")) {
        return Promise.resolve(gql({ mealPlan: plan }));
      }
      return Promise.resolve(
        gql({ mealPlans: { items: [plan], pageInfo: { pageNumber: 1, pageSize: 1000, totalCount: 1 } } })
      );
    });

    renderPage();
    await waitFor(() =>
      expect(screen.getByText(/Household invitations/)).toBeInTheDocument()
    );
    expect(screen.getByText(/Inviter invited you/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /accept/i }));
    await waitFor(() => {
      const calls = mockFetch.mock.calls.map(([, init]) =>
        JSON.parse((init as RequestInit).body as string)
      );
      expect(
        calls.some(
          (b) =>
            b.query.includes("acceptHouseholdInvite") &&
            b.variables.inviteId === "44"
        )
      ).toBe(true);
    });
  });
});
