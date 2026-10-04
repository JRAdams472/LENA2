import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import AllergenSuggestionsPage from "@/app/inventory/allergen-suggestions/page";

const mockFetch = global.fetch as jest.Mock;

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
      <AllergenSuggestionsPage />
    </QueryClientProvider>
  );
}

const pendingRow = {
  id: "5",
  recipeId: "10",
  recipeName: "Enchilada Casserole",
  targetKind: "ingredient",
  ingredientId: "51",
  ingredientName: "flour",
  itemId: null,
  itemName: null,
  allergen: { id: "2", name: "wheat", description: null, isActive: true },
  kind: "contains",
  rationale: "flour is wheat",
  status: "pending",
  reviewedAt: null,
};

const acceptedRow = { ...pendingRow, id: "6", status: "accepted" };

const gqlRecipe = {
  id: "10",
  name: "Enchilada Casserole",
  description: null,
  servings: 4,
  prepTimeMinutes: null,
  cookTimeMinutes: null,
  isFavorite: false,
  items: [],
  steps: [],
  selectionCount: 0,
  personalSelectionCount: 0,
  myRating: null,
  averageRating: null,
  ratingCount: 0,
  categories: [],
  allergens: [],
  allergyWarnings: [],
};

function dispatch(suggestions: object[]) {
  mockFetch.mockImplementation((_url: string, init: RequestInit) => {
    const body = JSON.parse(init.body as string);
    const q: string = body.query;
    if (q.includes("aiAvailable")) return Promise.resolve(gql({ aiAvailable: true }));
    if (q.includes("suggestRecipeAllergens"))
      return Promise.resolve(gql({ suggestRecipeAllergens: [pendingRow] }));
    if (q.includes("acceptAllergenSuggestion"))
      return Promise.resolve(gql({ acceptAllergenSuggestion: acceptedRow }));
    if (q.includes("dismissAllergenSuggestion"))
      return Promise.resolve(gql({ dismissAllergenSuggestion: { ...pendingRow, status: "dismissed" } }));
    if (q.includes("allergenSuggestions"))
      return Promise.resolve(gql({ allergenSuggestions: suggestions }));
    if (q.includes("recipes"))
      return Promise.resolve(
        gql({ recipes: { items: [gqlRecipe], pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 } } })
      );
    return Promise.reject(new Error(`unexpected query: ${q.slice(0, 80)}`));
  });
}

describe("AllergenSuggestionsPage", () => {
  beforeEach(() => mockFetch.mockReset());

  it("lists pending suggestions with accept/dismiss actions", async () => {
    dispatch([pendingRow]);
    renderPage();
    expect(await screen.findByText("flour")).toBeInTheDocument();
    expect(screen.getByText("wheat")).toBeInTheDocument();
    expect(screen.getByText("Enchilada Casserole")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Accept" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Dismiss" })).toBeInTheDocument();
    // Query asks for pending rows.
    expect(
      getBodies().some(
        (b) => b.query.includes("allergenSuggestions") && b.variables.status === "pending"
      )
    ).toBe(true);
  });

  it("accept mutation writes the flag via the resolver", async () => {
    dispatch([pendingRow]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Accept" }));
    await waitFor(() =>
      expect(
        getBodies().some(
          (b) => b.query.includes("acceptAllergenSuggestion") && b.variables.id === "5"
        )
      ).toBe(true)
    );
  });

  it("dismiss mutation marks the row dismissed", async () => {
    dispatch([pendingRow]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Dismiss" }));
    await waitFor(() =>
      expect(
        getBodies().some(
          (b) => b.query.includes("dismissAllergenSuggestion") && b.variables.id === "5"
        )
      ).toBe(true)
    );
  });

  it("runs the suggester for the picked recipe", async () => {
    dispatch([]);
    renderPage();
    const input = await screen.findByLabelText("Recipe");
    fireEvent.change(input, { target: { value: "Enchilada" } });
    fireEvent.click(await screen.findByRole("option", { name: "Enchilada Casserole" }));
    fireEvent.click(screen.getByRole("button", { name: /Suggest flags/ }));
    await waitFor(() =>
      expect(
        getBodies().some(
          (b) => b.query.includes("suggestRecipeAllergens") && b.variables.recipeId === "10"
        )
      ).toBe(true)
    );
  });

  it("reviewed rows render without action buttons", async () => {
    dispatch([acceptedRow]);
    renderPage();
    // Switch the filter to All so the accepted row is visible.
    fireEvent.click(await screen.findByRole("button", { name: "All" }));
    expect(await screen.findByText("flour")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Accept" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Dismiss" })).not.toBeInTheDocument();
  });
});
