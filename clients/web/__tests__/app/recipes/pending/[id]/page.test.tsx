import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import PendingRecipeDetailPage from "@/app/recipes/pending/[id]/page";

jest.mock("../../../../../app/auth/useMe");
import { useMe } from "../../../../../app/auth/useMe";

const mockedUseMe = useMe as jest.Mock;

const mockPush = jest.fn();
const mockFetch = global.fetch as jest.Mock;

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: mockPush })),
  useParams: jest.fn(() => ({ id: "1" })),
  usePathname: jest.fn(() => "/recipes/pending/1"),
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

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <PendingRecipeDetailPage />
    </QueryClientProvider>
  );
}

function meReturn(isAdmin: boolean, isLoading = false) {
  return {
    me: isAdmin ? { userID: 1, role: "admin" } : { userID: 2, role: "member" },
    isAdmin,
    isLoading,
    error: null,
    refetch: jest.fn(),
  };
}

const draft = {
  name: "Draft Pasta",
  description: "A draft",
  servings: 2,
  prepTimeMinutes: 10,
  cookTimeMinutes: 20,
  sourceHint: null,
  items: [
    {
      ingredient: "flour",
      quantity: 2,
      unit: "cup",
      section: null,
      notes: null,
      isOptional: false,
    },
  ],
  steps: [{ stepNumber: 1, instruction: "Mix" }],
};

const recipeImport = {
  id: "1",
  status: "reviewing",
  sourceFilename: "recipe-scan.png",
  ocrText: null,
  draft,
  review: null,
  recipe: null,
  profanityFlag: false,
  profanityReason: null,
  errorMessage: null,
};

const updatedRecipeImport = {
  ...recipeImport,
  status: "ready",
};

const reviewWithSuggestions = {
  pageId: null,
  name: "Draft Pasta",
  description: "A draft",
  servings: 2,
  prepTimeMinutes: 10,
  cookTimeMinutes: 20,
  sourceHint: null,
  items: [
    {
      draftItem: {
        ingredient: "flour",
        quantity: 2,
        unit: "cup",
        section: null,
        notes: null,
        isOptional: false,
      },
      itemId: null,
      itemKind: null,
      itemName: null,
      unit: "cup",
      unitId: null,
      confidence: 0.92,
      suggestions: [
        { id: "10", name: "Flour, All-Purpose", kind: "item", score: 0.92 },
        { id: "5", name: "flour", kind: "ingredient", score: 0.8 },
      ],
      status: "suggested",
      notes: null,
      approved: false,
    },
  ],
  steps: [{ stepNumber: 1, instruction: "Mix" }],
  approved: false,
};

const reviewingImport = {
  ...recipeImport,
  review: reviewWithSuggestions,
};

const units = [
  { id: "1", name: "cup", abbreviation: "c", kind: "volume", isActive: true },
  { id: "2", name: "each", abbreviation: null, kind: "count", isActive: true },
  { id: "3", name: "to taste", abbreviation: null, kind: "count", isActive: true },
];

beforeEach(() => {
  mockFetch.mockReset();
  mockPush.mockReset();
});

describe("pending recipe detail page", () => {
  it("shows loading while me is loading", () => {
    mockedUseMe.mockReturnValue(meReturn(true, true));
    renderPage();
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });

  it("shows Forbidden for non-admins", () => {
    mockedUseMe.mockReturnValue(meReturn(false));
    renderPage();
    expect(screen.getByText(/Forbidden/i)).toBeInTheDocument();
  });

  it("shows not found when the import is missing", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation(() =>
      Promise.resolve(gql({ recipeImport: null }))
    );
    renderPage();
    await waitFor(() =>
      expect(screen.getByText(/Recipe import not found/i)).toBeInTheDocument()
    );
  });

  it("renders the import and draft data", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation(() =>
      Promise.resolve(gql({ recipeImport }))
    );
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );
    expect(screen.getByText(/recipe-scan\.png/i)).toBeInTheDocument();
    expect(screen.getByLabelText("Servings")).toHaveValue(2);
    expect(screen.getByDisplayValue("Mix")).toBeInTheDocument();
  });

  it("saves the review", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve(gql({ updateRecipeImport: updatedRecipeImport }));
      }
      return Promise.resolve(gql({ recipeImport }));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );

    fireEvent.change(screen.getByLabelText("Recipe Name"), { target: { value: "Pasta" } });
    fireEvent.click(screen.getByRole("button", { name: /save review/i }));
    await waitFor(() =>
      expect(getBodies().some((b) => b.query.includes("updateRecipeImport"))).toBe(true)
    );
  });

  it("approves the import", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("approveRecipeImport")) {
        return Promise.resolve(
          gql({
            approveRecipeImport: {
              id: "10",
              name: "Pasta",
              description: "",
              servings: 2,
              prepTimeMinutes: 10,
              cookTimeMinutes: 20,
              isFavorite: false,
              isActive: true,
              items: [],
              steps: [],
              myRating: null,
              averageRating: null,
              ratingCount: 0,
            },
          })
        );
      }
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve(gql({ updateRecipeImport: updatedRecipeImport }));
      }
      return Promise.resolve(gql({ recipeImport }));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );
    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    await waitFor(() =>
      expect(getBodies().some((b) => b.query.includes("approveRecipeImport"))).toBe(true)
    );
    // approve persists edits first — update must precede approve
    const bodies = getBodies();
    const updateIdx = bodies.findIndex((b) => b.query.includes("updateRecipeImport"));
    const approveIdx = bodies.findIndex((b) => b.query.includes("approveRecipeImport"));
    expect(updateIdx).toBeGreaterThanOrEqual(0);
    expect(updateIdx).toBeLessThan(approveIdx);
    expect(mockPush).toHaveBeenCalledWith("/recipes/pending");
  });

  it("rejects the import", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("rejectRecipeImport")) {
        return Promise.resolve(gql({ rejectRecipeImport: { ...recipeImport, status: "rejected" } }));
      }
      return Promise.resolve(gql({ recipeImport }));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );
    fireEvent.click(screen.getByRole("button", { name: /^reject$/i }));
    fireEvent.click(screen.getByRole("button", { name: /yes, reject it/i }));
    await waitFor(() =>
      expect(getBodies().some((b) => b.query.includes("rejectRecipeImport"))).toBe(true)
    );
    expect(mockPush).toHaveBeenCalledWith("/recipes/pending");
  });

  it("cancelling the reject dialog does not reject", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation(() => Promise.resolve(gql({ recipeImport })));
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );
    fireEvent.click(screen.getByRole("button", { name: /^reject$/i }));
    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(getBodies().some((b) => b.query.includes("rejectRecipeImport"))).toBe(false);
  });

  it("retries a failed import", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    const failedImport = { ...recipeImport, status: "failed" };
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("retryRecipeImport")) {
        return Promise.resolve(gql({ retryRecipeImport: { ...failedImport, status: "pending" } }));
      }
      return Promise.resolve(gql({ recipeImport: failedImport }));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));
    await waitFor(() =>
      expect(getBodies().some((b) => b.query.includes("retryRecipeImport"))).toBe(true)
    );
  });

  it("disables retry while the import is under review", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation(() => Promise.resolve(gql({ recipeImport })));
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );
    expect(screen.getByRole("button", { name: /retry/i })).toBeDisabled();
  });

  it("adds and removes ingredients and steps", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve(gql({ updateRecipeImport: updatedRecipeImport }));
      }
      return Promise.resolve(gql({ recipeImport }));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );

    fireEvent.click(screen.getByRole("button", { name: /add ingredient/i }));
    fireEvent.click(screen.getByRole("button", { name: /add step/i }));
    fireEvent.click(screen.getAllByRole("button", { name: /remove ingredient/i })[0]);
    fireEvent.click(screen.getAllByRole("button", { name: /remove step/i })[0]);

    fireEvent.click(screen.getByRole("button", { name: /save review/i }));
    await waitFor(() =>
      expect(getBodies().some((b) => b.query.includes("updateRecipeImport"))).toBe(true)
    );
  });

  it("shows a save error when the API fails", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve({
          ok: false,
          status: 500,
          headers: { get: () => "text/plain" },
          text: async () => "Save failed",
        });
      }
      return Promise.resolve(gql({ recipeImport }));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );
    fireEvent.click(screen.getByRole("button", { name: /save review/i }));
    await waitFor(() =>
      expect(screen.getByText(/Save failed/i)).toBeInTheDocument()
    );
  });

  it("resolves an ingredient by picking a suggestion chip", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve(gql({ updateRecipeImport: updatedRecipeImport }));
      }
      if (body.query?.includes("recipeImport")) {
        return Promise.resolve(gql({ recipeImport: reviewingImport }));
      }
      if (body.query?.includes("units")) {
        return Promise.resolve(gql({ units }));
      }
      if (body.query?.includes("items(")) {
        return Promise.resolve(gql({ items: { items: [] } }));
      }
      return Promise.resolve(gql({}));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );

    // status chip + unresolved checkbox are visible
    expect(screen.getByText("suggested")).toBeInTheDocument();
    const resolved = screen.getByRole("checkbox", { name: /resolved/i });
    expect(resolved).not.toBeChecked();

    // picking an item suggestion fills the catalog item and checks Resolved
    fireEvent.click(screen.getByText(/Flour, All-Purpose/));
    await waitFor(() => expect(resolved).toBeChecked());

    fireEvent.click(screen.getByRole("button", { name: /save review/i }));
    await waitFor(() => {
      const update = getBodies().find((b) => b.query.includes("updateRecipeImport"));
      expect(update).toBeTruthy();
      expect(update.variables.input.items[0].itemId).toBe("10");
      expect(update.variables.input.items[0].itemKind).toBe("item");
      expect(update.variables.input.items[0].itemName).toBe("Flour, All-Purpose");
      expect(update.variables.input.items[0].approved).toBe(true);
    });
  });

  it("resolves an ingredient by picking a generic-ingredient chip", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve(gql({ updateRecipeImport: updatedRecipeImport }));
      }
      if (body.query?.includes("recipeImport")) {
        return Promise.resolve(gql({ recipeImport: reviewingImport }));
      }
      if (body.query?.includes("units")) {
        return Promise.resolve(gql({ units }));
      }
      if (body.query?.includes("items(")) {
        return Promise.resolve(gql({ items: { items: [] } }));
      }
      return Promise.resolve(gql({}));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );

    const resolved = screen.getByRole("checkbox", { name: /resolved/i });
    expect(resolved).not.toBeChecked();

    // the fixture has a "flour" suggestion with kind "ingredient" (id "5") —
    // clicking it must bind the ingredient id, not just fill the search box.
    fireEvent.click(screen.getByText(/flour \(ingredient\)/));
    await waitFor(() => expect(resolved).toBeChecked());

    fireEvent.click(screen.getByRole("button", { name: /save review/i }));
    await waitFor(() => {
      const update = getBodies().find((b) => b.query.includes("updateRecipeImport"));
      expect(update).toBeTruthy();
      expect(update.variables.input.items[0].itemId).toBe("5");
      expect(update.variables.input.items[0].itemKind).toBe("ingredient");
      expect(update.variables.input.items[0].itemName).toBe("flour");
      expect(update.variables.input.items[0].approved).toBe(true);
    });
  });

  it("lets the reviewer pick 'to taste' as the unit", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve(gql({ updateRecipeImport: updatedRecipeImport }));
      }
      if (body.query?.includes("recipeImport")) {
        return Promise.resolve(gql({ recipeImport: reviewingImport }));
      }
      if (body.query?.includes("units")) {
        return Promise.resolve(gql({ units }));
      }
      if (body.query?.includes("items(")) {
        return Promise.resolve(gql({ items: { items: [] } }));
      }
      return Promise.resolve(gql({}));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );

    const unitInput = screen.getByLabelText("Unit");
    fireEvent.mouseDown(unitInput);
    const option = await screen.findByRole("option", { name: "to taste" });
    fireEvent.click(option);

    fireEvent.click(screen.getByRole("button", { name: /save review/i }));
    await waitFor(() => {
      const update = getBodies().find((b) => b.query.includes("updateRecipeImport"));
      expect(update).toBeTruthy();
      expect(update.variables.input.items[0].unit).toBe("to taste");
      expect(update.variables.input.items[0].unitId).toBe("3");
    });
  });

  it("fills the catalog item from search results", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    const searchItem = {
      id: "42",
      name: "Semolina Flour",
      brand: null,
      category: null,
      upc12: null,
      upc14: null,
      unit: "each",
      description: null,
      nutrients: [],
      flavors: [],
      selectionCount: 0,
      personalSelectionCount: 0,
    };
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve(gql({ updateRecipeImport: updatedRecipeImport }));
      }
      if (body.query?.includes("recipeImport")) {
        return Promise.resolve(gql({ recipeImport: reviewingImport }));
      }
      if (body.query?.includes("units")) {
        return Promise.resolve(gql({ units }));
      }
      if (body.query?.includes("items(")) {
        return Promise.resolve(gql({ items: { items: [searchItem] } }));
      }
      return Promise.resolve(gql({}));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );

    const itemInput = screen.getByLabelText("Catalog Item");
    itemInput.focus();
    fireEvent.change(itemInput, { target: { value: "semo" } });
    const option = await screen.findByRole("option", { name: "Semolina Flour" }, { timeout: 3000 });
    fireEvent.click(option);

    fireEvent.click(screen.getByRole("button", { name: /save review/i }));
    await waitFor(() => {
      const update = getBodies().find((b) => b.query.includes("updateRecipeImport"));
      expect(update).toBeTruthy();
      expect(update.variables.input.items[0].itemId).toBe("42");
      expect(update.variables.input.items[0].approved).toBe(true);
    });
  });

  it("setting qty to 0 offers to remove the row", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve(gql({ updateRecipeImport: updatedRecipeImport }));
      }
      return Promise.resolve(gql({ recipeImport }));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );

    fireEvent.change(screen.getByLabelText("Qty"), { target: { value: "0" } });
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/Remove this ingredient/i)).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: /yes, remove/i }));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
    );
    await waitFor(() =>
      expect(screen.queryByLabelText("Qty")).not.toBeInTheDocument()
    );

    fireEvent.click(screen.getByRole("button", { name: /save review/i }));
    await waitFor(() => {
      const update = getBodies().find((b) => b.query.includes("updateRecipeImport"));
      expect(update).toBeTruthy();
      expect(update.variables.input.items).toHaveLength(0);
    });
  });

  it("setting qty to 0 and declining restores 1", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("updateRecipeImport")) {
        return Promise.resolve(gql({ updateRecipeImport: updatedRecipeImport }));
      }
      return Promise.resolve(gql({ recipeImport }));
    });
    renderPage();
    await waitFor(() =>
      expect(screen.getByDisplayValue("Draft Pasta")).toBeInTheDocument()
    );

    fireEvent.change(screen.getByLabelText("Qty"), { target: { value: "0" } });
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /no, keep at 1/i }));

    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
    );
    await waitFor(() =>
      expect(screen.getByLabelText("Qty")).toHaveValue(1)
    );

    fireEvent.click(screen.getByRole("button", { name: /save review/i }));
    await waitFor(() => {
      const update = getBodies().find((b) => b.query.includes("updateRecipeImport"));
      expect(update).toBeTruthy();
      expect(update.variables.input.items[0].quantity).toBe(1);
    });
  });
});
