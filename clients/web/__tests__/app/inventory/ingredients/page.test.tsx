import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import IngredientsPage from "@/app/inventory/ingredients/page";

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
      <IngredientsPage />
    </QueryClientProvider>
  );
}

const corn = {
  id: "7",
  name: "corn",
  category: { id: "3", name: "Vegetables", description: null, isActive: true, isProtein: false },
  defaultUnit: "cup",
  isActive: true,
};

const beans = {
  id: "9",
  name: "black beans",
  category: null,
  defaultUnit: null,
  isActive: true,
};

function ingredientsPage(items = [corn, beans]) {
  return {
    ingredients: {
      items,
      pageInfo: { pageNumber: 1, pageSize: 25, totalCount: items.length },
    },
  };
}

function mockAll() {
  mockFetch.mockImplementation((_, init) => {
    const body = JSON.parse((init as RequestInit).body as string);
    if (body.query.includes("mergeIngredient")) {
      return Promise.resolve(gql({ mergeIngredient: true }));
    }
    if (body.query.includes("createIngredient")) {
      return Promise.resolve(gql({ createIngredient: corn }));
    }
    if (body.query.includes("updateIngredient")) {
      return Promise.resolve(gql({ updateIngredient: corn }));
    }
    if (body.query.includes("getOrCreateIngredient")) {
      return Promise.resolve(gql({ getOrCreateIngredient: corn }));
    }
    if (body.query.includes("categories")) {
      return Promise.resolve(
        gql({
          categories: [
            { id: "3", name: "Vegetables", description: null, isActive: true, isProtein: false },
          ],
        })
      );
    }
    if (body.query.includes("ingredients")) {
      return Promise.resolve(gql(ingredientsPage()));
    }
    return Promise.resolve(gql({}));
  });
}

describe("ingredients admin page", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockAll();
  });

  it("lists ingredients with category", async () => {
    renderPage();
    await waitFor(() => screen.getByText("corn"));
    expect(screen.getByText("black beans")).toBeInTheDocument();
    expect(screen.getByText("Vegetables")).toBeInTheDocument();
  });

  it("sends the search term to the ingredients query", async () => {
    renderPage();
    await waitFor(() => screen.getByText("corn"));
    fireEvent.change(screen.getByLabelText("Search"), { target: { value: "cor" } });
    await waitFor(() => {
      const searchCall = getBodies().find(
        (b) => b.query.includes("ingredients(") && b.variables?.search === "cor"
      );
      expect(searchCall).toBeTruthy();
    });
  });

  it("creates an ingredient with name, category and unit", async () => {
    renderPage();
    await waitFor(() => screen.getByText("corn"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Name"), {
      target: { value: "quinoa" },
    });
    fireEvent.change(within(dialog).getByLabelText("Default Unit"), {
      target: { value: "cup" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: /save/i }));

    await waitFor(() => {
      const createCall = getBodies().find((b) => b.query.includes("createIngredient"));
      expect(createCall).toBeTruthy();
      expect(createCall.variables.input.name).toBe("quinoa");
      expect(createCall.variables.input.defaultUnit).toBe("cup");
    });
  });

  it("edits an ingredient via the row edit action", async () => {
    renderPage();
    await waitFor(() => screen.getByText("corn"));

    const row = screen.getByText("corn").closest("tr")!;
    fireEvent.click(within(row).getByRole("button", { name: /edit/i }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Name"), {
      target: { value: "sweet corn" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: /save/i }));

    await waitFor(() => {
      const updateCall = getBodies().find((b) => b.query.includes("updateIngredient"));
      expect(updateCall).toBeTruthy();
      expect(updateCall.variables.id).toBe("7");
      expect(updateCall.variables.input.name).toBe("sweet corn");
    });
  });

  it("merges a source ingredient into the picked survivor", async () => {
    renderPage();
    await waitFor(() => screen.getByText("corn"));

    const row = screen.getByText("corn").closest("tr")!;
    fireEvent.click(within(row).getByRole("button", { name: /merge into/i }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Every recipe line");

    const picker = within(dialog).getByLabelText("Surviving ingredient");
    picker.focus();
    fireEvent.change(picker, { target: { value: "black" } });
    const option = await screen.findByRole("option", { name: "black beans" });
    fireEvent.click(option);

    fireEvent.click(within(dialog).getByRole("button", { name: /^merge$/i }));

    await waitFor(() => {
      const mergeCall = getBodies().find((b) => b.query.includes("mergeIngredient"));
      expect(mergeCall).toBeTruthy();
      expect(mergeCall.variables).toEqual({ fromId: "7", intoId: "9" });
    });
  });
});
