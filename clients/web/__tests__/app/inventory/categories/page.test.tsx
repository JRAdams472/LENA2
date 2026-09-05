import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import CategoriesPage from "@/app/inventory/categories/page";

const mockFetch = global.fetch as jest.Mock;

function gql(data: object) {
  return {
    ok: true,
    status: 200,
    headers: { get: () => "application/json" },
    json: async () => ({ data }),
  };
}

const category = {
  id: "1",
  name: "Dairy",
  description: "Milk and cheese",
  isActive: true,
};

function lastRequestBody() {
  const [, init] = mockFetch.mock.calls[mockFetch.mock.calls.length - 1];
  return JSON.parse((init as RequestInit).body as string);
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <CategoriesPage />
    </QueryClientProvider>
  );
}

describe("categories page", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    jest.spyOn(window, "confirm").mockReturnValue(true);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("lists categories in the table", async () => {
    mockFetch.mockResolvedValueOnce(gql({ categories: [category] }));
    renderPage();

    await waitFor(() =>
      expect(screen.getByText("Dairy")).toBeInTheDocument()
    );
    expect(screen.getByText("Milk and cheese")).toBeInTheDocument();
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:5059/graphql",
      expect.objectContaining({ method: "POST" })
    );
    expect(lastRequestBody().query).toContain("categories");
  });

  it("creates a category through the dialog", async () => {
    mockFetch
      .mockResolvedValueOnce(gql({ categories: [category] }))
      .mockResolvedValueOnce(
        gql({ createCategory: { ...category, id: "2", name: "Bakery" } })
      )
      .mockResolvedValueOnce(gql({ categories: [category] }));
    renderPage();

    await waitFor(() => screen.getByText("Dairy"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Bakery" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const bodies = mockFetch.mock.calls.map((c) =>
        JSON.parse((c[1] as RequestInit).body as string)
      );
      expect(
        bodies.some(
          (b) =>
            b.query.includes("createCategory") &&
            b.variables.input.name === "Bakery"
        )
      ).toBe(true);
    });
  });

  it("edits a category through the dialog", async () => {
    mockFetch
      .mockResolvedValueOnce(gql({ categories: [category] }))
      .mockResolvedValueOnce(gql({ updateCategory: category }))
      .mockResolvedValueOnce(gql({ categories: [category] }));
    renderPage();

    await waitFor(() => screen.getByText("Dairy"));
    const row = screen.getByText("Dairy").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);

    const nameField = await screen.findByLabelText("Name");
    expect(nameField).toHaveValue("Dairy");
    fireEvent.change(nameField, { target: { value: "Dairy & Eggs" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const bodies = mockFetch.mock.calls.map((c) =>
        JSON.parse((c[1] as RequestInit).body as string)
      );
      expect(
        bodies.some(
          (b) =>
            b.query.includes("updateCategory") &&
            b.variables.id === "1" &&
            b.variables.input.name === "Dairy & Eggs"
        )
      ).toBe(true);
    });
  });

  it("deletes a category after confirmation", async () => {
    mockFetch
      .mockResolvedValueOnce(gql({ categories: [category] }))
      .mockResolvedValueOnce(gql({ deleteCategory: true }))
      .mockResolvedValueOnce(gql({ categories: [] }));
    renderPage();

    await waitFor(() => screen.getByText("Dairy"));
    const row = screen.getByText("Dairy").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);

    await waitFor(() => {
      const bodies = mockFetch.mock.calls.map((c) =>
        JSON.parse((c[1] as RequestInit).body as string)
      );
      expect(
        bodies.some(
          (b) =>
            b.query.includes("deleteCategory") && b.variables.id === "1"
        )
      ).toBe(true);
    });
  });

  it("filters to active categories with the toggle", async () => {
    mockFetch
      .mockResolvedValueOnce(
        gql({
          categories: [
            category,
            { ...category, id: "2", name: "Retired", isActive: false },
          ],
        })
      )
      .mockResolvedValueOnce(
        gql({
          categories: [
            category,
            { ...category, id: "2", name: "Retired", isActive: false },
          ],
        })
      );
    renderPage();

    await waitFor(() => screen.getByText("Retired"));
    fireEvent.click(screen.getByLabelText("Active only"));

    await waitFor(() =>
      expect(screen.queryByText("Retired")).not.toBeInTheDocument()
    );
    expect(screen.getByText("Dairy")).toBeInTheDocument();
  });
});
