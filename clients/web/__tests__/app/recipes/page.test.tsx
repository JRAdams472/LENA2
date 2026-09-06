import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import RecipesPage from "@/app/recipes/page";
import RecipeDetailPage from "@/app/recipes/[id]/page";

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
});

describe("recipe detail page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("brands")) {
        return Promise.resolve(gql({ brands: [{ id: "1", name: "DairyCo" }] }));
      }
      if (body.query.includes("updateRecipe")) {
        return Promise.resolve(gql({ updateRecipe: recipe }));
      }
      if (body.query.includes("rateRecipe")) {
        return Promise.resolve(
          gql({ rateRecipe: { ...recipe, myRating: 5, ratingCount: 5 } })
        );
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
});
