import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import RecipeDetailPage from "@/app/recipes/[id]/page";

jest.mock("../../../app/auth/useMe");
import { useMe } from "../../../app/auth/useMe";

const mockedUseMe = useMe as jest.Mock;
const mockFetch = global.fetch as jest.Mock;

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: jest.fn() })),
  useParams: jest.fn(() => ({ id: "1" })),
  usePathname: jest.fn(() => "/recipes/1"),
}));

// AI suggestions hook into a client-side engine store — stub it quiet.
jest.mock("../../../lib/ai/engineStore", () => ({
  useLocalEngineReady: () => false,
}));
jest.mock("../../../lib/ai/suggest", () => ({ suggestPairings: jest.fn() }));

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

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <RecipeDetailPage />
    </QueryClientProvider>
  );
}

const deltaRow = {
  id: "7",
  stale: false,
  orphanedItemCount: 0,
  orphanedStepCount: 0,
  updatedAt: "2026-10-05T12:00:00Z",
  items: [
    {
      id: "11",
      kind: "substitute",
      recipeItemId: "10",
      item: null,
      ingredient: { id: "5", name: "oat milk" },
      quantity: null,
      unit: null,
      unitId: null,
      section: null,
      displayOrder: null,
      notes: null,
      isOptional: null,
      orphaned: false,
    },
  ],
  steps: [],
};

const recipe = {
  id: "1",
  name: "Pasta",
  description: "Delicious",
  servings: 2,
  prepTimeMinutes: 10,
  cookTimeMinutes: 20,
  isFavorite: false,
  selectionCount: 0,
  personalSelectionCount: 0,
  myRating: null,
  averageRating: null,
  ratingCount: 0,
  categories: [],
  items: [
    {
      id: "10",
      quantity: 2,
      unit: "cup",
      section: null,
      displayOrder: 1,
      notes: null,
      isOptional: false,
      deltaKind: "substitute",
      ingredient: { id: "5", name: "oat milk" },
      item: null,
    },
    {
      id: "12",
      quantity: 1,
      unit: "tbsp",
      section: null,
      displayOrder: 2,
      notes: null,
      isOptional: false,
      deltaKind: null,
      ingredient: { id: "6", name: "olive oil" },
      item: null,
    },
  ],
  steps: [
    {
      id: "20",
      stepNumber: 1,
      instruction: "Boil water",
      durationMinutes: 5,
      stepType: "cook",
      isPassive: false,
      dependsOnStepNumber: null,
      appliance: null,
      deltaKind: "add",
    },
  ],
  householdDelta: deltaRow,
};

const memberWithHousehold = {
  me: { role: "member", household: { householdID: 9 } },
  isAdmin: false,
  isLoading: false,
};

function mockApi(overrides?: { recipe?: object }) {
  mockFetch.mockImplementation((_, init) => {
    const body = JSON.parse((init as RequestInit).body as string);
    if (body.query.includes("setRecipeDelta")) {
      return Promise.resolve(gql({ setRecipeDelta: deltaRow }));
    }
    if (body.query.includes("clearRecipeDelta")) {
      return Promise.resolve(gql({ clearRecipeDelta: true }));
    }
    if (body.query.includes("acknowledgeRecipeDelta")) {
      return Promise.resolve(
        gql({ acknowledgeRecipeDelta: { ...deltaRow, stale: false } })
      );
    }
    if (body.query.includes("recordView")) {
      return Promise.resolve(gql({ recordView: true }));
    }
    if (body.query.includes("units")) {
      return Promise.resolve(
        gql({
          units: [
            { id: "33", name: "cup", abbreviation: "c", kind: "volume", isActive: true },
          ],
        })
      );
    }
    if (body.query.includes("aiAvailable")) {
      return Promise.resolve(gql({ aiAvailable: false }));
    }
    if (body.query.includes("recipe(")) {
      return Promise.resolve(gql({ recipe: overrides?.recipe ?? recipe }));
    }
    return Promise.resolve(gql({ recipe: overrides?.recipe ?? recipe }));
  });
}

beforeEach(() => {
  mockFetch.mockReset();
  mockedUseMe.mockReturnValue(memberWithHousehold);
  mockApi();
});

afterEach(() => {
  jest.restoreAllMocks();
});

describe("recipe detail — household delta", () => {
  it("shows the household chip and the swapped badge on the tweaked line", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    // The chip and the selected toggle both carry the label.
    expect(screen.getAllByText("Household version").length).toBeGreaterThan(0);
    expect(screen.getByText("Swapped")).toBeInTheDocument();
    expect(screen.getByText("oat milk")).toBeInTheDocument();
    // Steps also carry badges.
    expect(screen.getByText("Added")).toBeInTheDocument();
  });

  it("offers Tweak on lines and steps for a plain member", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    const tweaks = screen.getAllByRole("button", { name: "Tweak" });
    expect(tweaks.length).toBeGreaterThanOrEqual(2);
    // Members never see canonical edit controls.
    expect(screen.queryByRole("button", { name: "Delete" })).toBeNull();
    expect(screen.queryByText("Add Ingredient")).toBeNull();
  });

  it("shows the stale banner and acknowledges it", async () => {
    mockApi({
      recipe: {
        ...recipe,
        householdDelta: { ...deltaRow, stale: true, orphanedItemCount: 1 },
      },
    });
    renderPage();
    await waitFor(() =>
      expect(
        screen.getByText(/original recipe changed after these tweaks/)
      ).toBeInTheDocument()
    );
    expect(screen.getByText(/no longer apply/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Mark reviewed" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) =>
            b.query.includes("acknowledgeRecipeDelta") &&
            b.variables.recipeId === "1"
        )
      ).toBe(true);
    });
  });

  it("dismisses the stale banner without acknowledging", async () => {
    mockApi({
      recipe: {
        ...recipe,
        householdDelta: { ...deltaRow, stale: true },
      },
    });
    renderPage();
    await waitFor(() =>
      expect(
        screen.getByText(/original recipe changed after these tweaks/)
      ).toBeInTheDocument()
    );
    fireEvent.click(screen.getByLabelText("Dismiss"));
    await waitFor(() =>
      expect(
        screen.queryByText(/original recipe changed after these tweaks/)
      ).toBeNull()
    );
    expect(
      getBodies().some((b) => b.query.includes("acknowledgeRecipeDelta"))
    ).toBe(false);
  });

  it("switches to the canonical view", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    mockFetch.mockClear();
    fireEvent.click(screen.getByRole("button", { name: "Original recipe" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) => b.query.includes("recipe(") && b.variables.view === "canonical"
        )
      ).toBe(true);
    });
  });

  it("saves the change set through setRecipeDelta", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    // Open the tweak editor on the untouched second line and remove it.
    const oliveRow = screen.getByText("olive oil").closest("tr") as HTMLElement;
    fireEvent.click(
      Array.from(oliveRow.querySelectorAll("button")).find(
        (b) => b.textContent === "Tweak"
      ) as HTMLElement
    );
    fireEvent.click(screen.getByLabelText("Remove"));
    fireEvent.click(screen.getByRole("button", { name: "Apply tweak" }));
    await waitFor(() =>
      expect(screen.getByText("Unsaved changes")).toBeInTheDocument()
    );

    fireEvent.click(screen.getByRole("button", { name: "Save tweaks" }));
    await waitFor(() => {
      const save = getBodies().find((b) => b.query.includes("setRecipeDelta"));
      expect(save).toBeTruthy();
      expect(save.variables.recipeId).toBe("1");
      expect(save.variables.items).toContainEqual(
        expect.objectContaining({ recipeItemId: "12", kind: "remove" })
      );
      // The existing substitute row is carried through verbatim.
      expect(save.variables.items).toContainEqual(
        expect.objectContaining({ recipeItemId: "10", kind: "substitute" })
      );
    });
  });

  it("clears every tweak via clearRecipeDelta", async () => {
    jest.spyOn(window, "confirm").mockReturnValue(true);
    renderPage();
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Clear all tweaks" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) =>
            b.query.includes("clearRecipeDelta") &&
            b.variables.recipeId === "1"
        )
      ).toBe(true);
    });
  });

  it("hides the tweaks panel for a user with no household", async () => {
    mockedUseMe.mockReturnValue({
      me: { role: "member", household: null },
      isAdmin: false,
      isLoading: false,
    });
    renderPage();
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    expect(screen.queryByText("Household tweaks")).toBeNull();
    expect(screen.queryByRole("button", { name: "Tweak" })).toBeNull();
  });
});

describe("recipe detail — cache regressions", () => {
  it("updates the categories chip after a category pick (view-keyed cache)", async () => {
    mockedUseMe.mockReturnValue({
      me: { role: "admin", household: { householdID: 9 } },
      isAdmin: true,
      isLoading: false,
    });
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("recipeCategoryGroups")) {
        return Promise.resolve(
          gql({
            recipeCategoryGroups: [
              {
                id: "1",
                name: "Course",
                exclusive: true,
                displayOrder: 1,
                categories: [
                  {
                    id: "4",
                    name: "Dinner",
                    group: { id: "1", name: "Course", exclusive: true, displayOrder: 1 },
                  },
                  {
                    id: "5",
                    name: "Lunch",
                    group: { id: "1", name: "Course", exclusive: true, displayOrder: 1 },
                  },
                ],
              },
            ],
          })
        );
      }
      if (body.query.includes("setRecipeCategories")) {
        recipe.categories = [
          {
            id: "4",
            name: "Dinner",
            group: { id: "1", name: "Course", exclusive: true, displayOrder: 1 },
          },
        ] as typeof recipe.categories;
        return Promise.resolve(
          gql({
            setRecipeCategories: { ...recipe },
          })
        );
      }
      if (body.query.includes("units")) {
        return Promise.resolve(gql({ units: [] }));
      }
      if (body.query.includes("aiAvailable")) {
        return Promise.resolve(gql({ aiAvailable: false }));
      }
      if (body.query.includes("recordView")) {
        return Promise.resolve(gql({ recordView: true }));
      }
      return Promise.resolve(gql({ recipe }));
    });
    renderPage();
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    fireEvent.click(await screen.findByRole("radio", { name: "Dinner" }));
    await waitFor(() =>
      expect(screen.getByText("Course: Dinner")).toBeInTheDocument()
    );
  });
});
