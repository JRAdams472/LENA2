import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import RecipesPage from "@/app/recipes/page";
import RecipeDetailPage from "@/app/recipes/[id]/page";

jest.mock("../../../app/auth/useMe");
import { useMe } from "../../../app/auth/useMe";

const mockedUseMe = useMe as jest.Mock;

const mockFetch = global.fetch as jest.Mock;

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: jest.fn() })),
  useParams: jest.fn(() => ({ id: "1" })),
  usePathname: jest.fn(() => "/recipes"),
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

const recipe = {
  id: "1",
  name: "Pasta",
  description: "Delicious",
  servings: 2,
  prepTimeMinutes: 10,
  cookTimeMinutes: 20,
  isFavorite: false,
  isActive: true,
  items: [],
  steps: [{ stepNumber: 1, instruction: "Boil water" }],
  myRating: 4,
  averageRating: 4.25,
  ratingCount: 4,
  categories: [
    {
      id: "22",
      name: "Italian",
      group: { id: "3", name: "Cuisine", exclusive: true, displayOrder: 6 },
    },
  ],
};

const categoryGroups = {
  recipeCategoryGroups: [
    {
      id: "3",
      name: "Cuisine",
      exclusive: true,
      displayOrder: 6,
      categories: [
        { id: "21", name: "Mexican", group: { id: "3", name: "Cuisine", exclusive: true, displayOrder: 6 } },
        { id: "22", name: "Italian", group: { id: "3", name: "Cuisine", exclusive: true, displayOrder: 6 } },
      ],
    },
    {
      id: "2",
      name: "Dish Type",
      exclusive: false,
      displayOrder: 2,
      categories: [
        { id: "40", name: "Soup", group: { id: "2", name: "Dish Type", exclusive: false, displayOrder: 2 } },
      ],
    },
  ],
};

beforeEach(() => {
  mockFetch.mockReset();
  jest.spyOn(window, "confirm").mockReturnValue(true);
});

afterEach(() => {
  jest.restoreAllMocks();
});

describe("recipes page", () => {
  beforeEach(() => {
    mockedUseMe.mockReturnValue({ me: { role: "admin" }, isAdmin: true, isLoading: false });
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createRecipe")) {
        return Promise.resolve(gql({ createRecipe: { ...recipe, id: "2", name: "Pizza" } }));
      }
      if (body.query.includes("updateRecipe")) {
        return Promise.resolve(gql({ updateRecipe: { ...recipe, name: "Pasta Updated" } }));
      }
      if (body.query.includes("deleteRecipe")) {
        return Promise.resolve(gql({ deleteRecipe: true }));
      }
      if (body.query.includes("recipeCategoryGroups")) {
        return Promise.resolve(gql(categoryGroups));
      }
      if (body.query.includes("recipe(")) {
        return Promise.resolve(gql({ recipe: recipe }));
      }
      return Promise.resolve(
        gql({
          recipes: {
            items: [recipe],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      );
    });
  });

  it("lists recipes", async () => {
    renderPage(<RecipesPage />);
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
  });

  it("creates a recipe", async () => {
    renderPage(<RecipesPage />);
    await waitFor(() => screen.getByText("Pasta"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Pizza" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createRecipe"))).toBe(true);
    });
  });

  it("edits a recipe", async () => {
    renderPage(<RecipesPage />);
    await waitFor(() => screen.getByText("Pasta"));
    const row = screen.getByText("Pasta").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Name"), { target: { value: "Pasta Updated" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateRecipe") && b.variables.id === "1")).toBe(true);
    });
  });

  it("deletes a recipe", async () => {
    renderPage(<RecipesPage />);
    await waitFor(() => screen.getByText("Pasta"));
    const row = screen.getByText("Pasta").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteRecipe") && b.variables.id === "1")).toBe(true);
    });
  });

  it("offers cocktail ideas to a 21+ user and applies the in-stock toggle", async () => {
    mockedUseMe.mockReturnValue({
      me: { role: "member", birthdate: "1985-06-20" },
      isAdmin: false,
      isLoading: false,
    });
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("aiAvailable")) {
        return Promise.resolve(gql({ aiAvailable: true }));
      }
      if (body.query.includes("suggestCocktails")) {
        return Promise.resolve(
          gql({
            suggestCocktails: [
              {
                recipe: { ...recipe, name: "Margarita" },
                reason: "citrus on hand",
                missingIngredients: [],
              },
            ],
          })
        );
      }
      if (body.query.includes("recipeCategoryGroups")) {
        return Promise.resolve(gql(categoryGroups));
      }
      return Promise.resolve(
        gql({
          recipes: {
            items: [recipe],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      );
    });

    renderPage(<RecipesPage />);
    fireEvent.click(await screen.findByRole("button", { name: /cocktail ideas/i }));
    await waitFor(() => screen.getByText("Margarita"));
    expect(screen.getByText("In stock")).toBeInTheDocument();
    expect(screen.getByText("citrus on hand")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText(/Only what I can make/i));
    await waitFor(() =>
      expect(
        getBodies().some(
          (b) => b.query.includes("suggestCocktails") && b.variables.inStockOnly === true
        )
      ).toBe(true)
    );
  });

  it("hides the semantic toggle when embeddings aren't configured", async () => {
    renderPage(<RecipesPage />);
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    expect(screen.queryByLabelText("Semantic")).not.toBeInTheDocument();
  });

  it("sends searchMode=semantic when the toggle is on", async () => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("semanticSearchAvailable")) {
        return Promise.resolve(gql({ semanticSearchAvailable: true }));
      }
      if (body.query.includes("recipeCategoryGroups")) {
        return Promise.resolve(gql(categoryGroups));
      }
      return Promise.resolve(
        gql({
          recipes: {
            items: [recipe],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      );
    });

    renderPage(<RecipesPage />);
    fireEvent.click(await screen.findByLabelText("Semantic"));
    fireEvent.change(screen.getByLabelText("Search"), { target: { value: "cozy stew" } });
    await waitFor(() =>
      expect(
        getBodies().some(
          (b) => b.query.includes("recipes(") && b.variables.searchMode === "semantic"
        )
      ).toBe(true)
    );
  });

  it("shows the describe hint instead of the list when semantic is on with no search", async () => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("semanticSearchAvailable")) {
        return Promise.resolve(gql({ semanticSearchAvailable: true }));
      }
      if (body.query.includes("recipeCategoryGroups")) {
        return Promise.resolve(gql(categoryGroups));
      }
      return Promise.resolve(
        gql({
          recipes: {
            items: [recipe],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      );
    });

    renderPage(<RecipesPage />);
    fireEvent.click(await screen.findByLabelText("Semantic"));
    await waitFor(() =>
      expect(screen.getByText(/Describe what you're in the mood for/)).toBeInTheDocument()
    );
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});

describe("recipe detail page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("searchBrands")) {
        return Promise.resolve(
          gql({ searchBrands: [{ id: "1", name: "DairyCo" }] })
        );
      }
      if (body.query.includes("updateRecipe")) {
        return Promise.resolve(gql({ updateRecipe: recipe }));
      }
      if (body.query.includes("rateRecipe")) {
        return Promise.resolve(
          gql({ rateRecipe: { ...recipe, myRating: 5, ratingCount: 5 } })
        );
      }
      if (body.query.includes("recordView")) {
        return Promise.resolve(gql({ recordView: true }));
      }
      if (body.query.includes("setRecipeCategories")) {
        return Promise.resolve(gql({ setRecipeCategories: recipe }));
      }
      if (body.query.includes("recipeCategoryGroups")) {
        return Promise.resolve(gql(categoryGroups));
      }
      if (body.query.includes("recipe(")) {
        return Promise.resolve(gql({ recipe: recipe }));
      }
      return Promise.resolve(gql({ recipe: recipe }));
    });
  });

  it("renders the recipe", async () => {
    renderPage(<RecipeDetailPage />);
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    expect(screen.getByText("Delicious")).toBeInTheDocument();
    expect(screen.getByText("1.")).toBeInTheDocument();
    expect(screen.getByText("Boil water")).toBeInTheDocument();
  });

  it("shows the rating summary and rates the recipe", async () => {
    renderPage(<RecipeDetailPage />);
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    expect(screen.getByText(/4\.3 avg · 4 ratings/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "5 Stars" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) =>
            b.query.includes("rateRecipe") &&
            b.variables.recipeId === "1" &&
            b.variables.rating === 5
        )
      ).toBe(true);
    });
  });

  it("adds a step", async () => {
    renderPage(<RecipeDetailPage />);
    await waitFor(() => screen.getByText("Pasta"));
    fireEvent.change(screen.getByLabelText("Step Number"), { target: { value: "2" } });
    fireEvent.change(screen.getByLabelText("Instruction"), { target: { value: "Drain pasta" } });
    fireEvent.click(screen.getByRole("button", { name: /add step/i }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateRecipe"))).toBe(true);
    });
  });

  it("deletes a step", async () => {
    renderPage(<RecipeDetailPage />);
    await waitFor(() => screen.getByText("Boil water"));
    const stepRow = screen.getByText("Boil water").closest("[class*=MuiBox-root]") as HTMLElement;
    const deleteButton = stepRow?.querySelectorAll("button")[1];
    if (deleteButton) fireEvent.click(deleteButton);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateRecipe"))).toBe(true);
    });
  });

  it("offers wine pairing suggestions for a 21+ user", async () => {
    mockedUseMe.mockReturnValue({
      me: { role: "member", birthdate: "1990-01-15" },
      isAdmin: false,
      isLoading: false,
    });
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("aiAvailable")) {
        return Promise.resolve(gql({ aiAvailable: true }));
      }
      if (body.query.includes("suggestPairings")) {
        return Promise.resolve(
          gql({
            suggestPairings: [
              { bottleId: "501", name: "Ridge 2019", reason: "tannins cut the fat", inCellar: true },
              { bottleId: null, name: "off-dry Riesling", reason: "acidity lifts the sauce", inCellar: false },
            ],
          })
        );
      }
      if (body.query.includes("recipeCategoryGroups")) {
        return Promise.resolve(gql(categoryGroups));
      }
      if (body.query.includes("recordView")) {
        return Promise.resolve(gql({ recordView: true }));
      }
      return Promise.resolve(gql({ recipe: recipe }));
    });

    renderPage(<RecipeDetailPage />);
    await waitFor(() => screen.getByText("Pasta"));
    fireEvent.click(await screen.findByRole("button", { name: /suggest wine pairing/i }));
    await waitFor(() => screen.getByText(/Ridge 2019 — tannins cut the fat/));
    expect(screen.getByText("In your cellar")).toBeInTheDocument();
    expect(screen.getByText(/off-dry Riesling/)).toBeInTheDocument();
    const call = getBodies().find((b) => b.query.includes("suggestPairings"))!;
    expect(call.variables.recipeId).toBe("1");
  });

  it("hides the pairing button without a 21+ birthdate", async () => {
    mockedUseMe.mockReturnValue({
      me: { role: "member", birthdate: null },
      isAdmin: false,
      isLoading: false,
    });
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("aiAvailable")) {
        return Promise.resolve(gql({ aiAvailable: true }));
      }
      if (body.query.includes("recipeCategoryGroups")) {
        return Promise.resolve(gql(categoryGroups));
      }
      if (body.query.includes("recordView")) {
        return Promise.resolve(gql({ recordView: true }));
      }
      return Promise.resolve(gql({ recipe: recipe }));
    });

    renderPage(<RecipeDetailPage />);
    await waitFor(() => screen.getByText("Pasta"));
    expect(
      screen.queryByRole("button", { name: /suggest wine pairing/i })
    ).not.toBeInTheDocument();
  });
});

describe("recipe categories", () => {
  beforeEach(() => {
    mockedUseMe.mockReturnValue({ me: { role: "admin" }, isAdmin: true, isLoading: false });
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("recordView")) {
        return Promise.resolve(gql({ recordView: true }));
      }
      if (body.query.includes("setRecipeCategories")) {
        return Promise.resolve(gql({ setRecipeCategories: recipe }));
      }
      if (body.query.includes("recipeCategoryGroups")) {
        return Promise.resolve(gql(categoryGroups));
      }
      if (body.query.includes("recipe(")) {
        return Promise.resolve(gql({ recipe: recipe }));
      }
      return Promise.resolve(
        gql({
          recipes: {
            items: [recipe],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      );
    });
  });

  it("renders the category filter bar on the recipes page", async () => {
    renderPage(<RecipesPage />);
    await waitFor(() => expect(screen.getByText("Pasta")).toBeInTheDocument());
    // One Select per group — outlined MUI labels render in a legend, so query by text.
    expect(screen.getAllByText("Cuisine").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Dish Type").length).toBeGreaterThan(0);
  });

  it("renders category chips and the picker on the detail page", async () => {
    renderPage(<RecipeDetailPage />);
    await waitFor(() => expect(screen.getByText("Cuisine: Italian")).toBeInTheDocument());
    // Exclusive group renders radios labeled "pick one".
    expect(screen.getByText(/pick one/i)).toBeInTheDocument();
    expect(screen.getByLabelText("Mexican")).toBeInTheDocument();
    // Non-exclusive group renders a checkbox.
    expect(screen.getByLabelText("Soup")).toBeInTheDocument();
  });

  it("records a view when the detail page opens", async () => {
    renderPage(<RecipeDetailPage />);
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) => b.query.includes("recordView") && b.variables?.entityType === "recipe"
        )
      ).toBe(true);
    });
  });

  it("assigns a category via setRecipeCategories", async () => {
    renderPage(<RecipeDetailPage />);
    await waitFor(() => screen.getByLabelText("Mexican"));
    fireEvent.click(screen.getByLabelText("Mexican"));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) => b.query.includes("setRecipeCategories") &&
            b.variables?.recipeId === "1" &&
            b.variables?.categoryIds?.includes("21")
        )
      ).toBe(true);
    });
  });

  it("shows members read-only chips and never fetches the picker taxonomy", async () => {
    mockedUseMe.mockReturnValue({ me: { role: "member" }, isAdmin: false, isLoading: false });
    renderPage(<RecipeDetailPage />);
    await waitFor(() => expect(screen.getByText("Cuisine: Italian")).toBeInTheDocument());
    expect(screen.queryByText(/pick one/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Mexican")).not.toBeInTheDocument();
    expect(
      getBodies().every((b) => !b.query.includes("recipeCategoryGroups"))
    ).toBe(true);
  });
});
