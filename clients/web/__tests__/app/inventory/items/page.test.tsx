import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import ItemsPage from "@/app/inventory/items/page";

jest.mock("../../../../app/auth/useMe");
import { useMe } from "../../../../app/auth/useMe";

const mockedUseMe = useMe as jest.Mock;

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
      <ItemsPage />
    </QueryClientProvider>
  );
}

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
  minQty: 1,
  purchaseAt: null,
  expiresAt: null,
  notes: null,
  isFavorite: false,
  item: { id: "1" },
};

describe("items page", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    jest.spyOn(window, "confirm").mockReturnValue(true);
    mockedUseMe.mockReturnValue({
      me: { userID: 1, role: "admin" },
      isAdmin: true,
      isLoading: false,
      error: null,
      refetch: jest.fn(),
    });

    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createItem")) {
        return Promise.resolve(
          gql({
            createItem: {
              ...item,
              id: "2",
              name: "Butter",
              brand: { id: "2", name: "DairyCo" },
              category: { id: "3", name: "Dairy", description: null },
            },
          })
        );
      }
      if (body.query.includes("updateItem")) {
        return Promise.resolve(gql({ updateItem: { ...item, name: "Whole Milk" } }));
      }
      if (body.query.includes("deleteItem")) {
        return Promise.resolve(gql({ deleteItem: true }));
      }
      if (body.query.includes("adjustUserItem")) {
        return Promise.resolve(gql({ adjustUserItem: { id: "10" } }));
      }
      if (body.query.includes("setItemFavorite")) {
        return Promise.resolve(gql({ setItemFavorite: true }));
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
      if (body.query.includes("frequentBrands")) {
        return Promise.resolve(
          gql({
            frequentBrands: [{ id: "2", name: "DairyCo" }],
          })
        );
      }
      return Promise.resolve(
        gql({
          searchBrands: [{ id: "2", name: "DairyCo" }],
          items: {
            items: [item],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      );
    });
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("lists items", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Milk")).toBeInTheDocument());
    expect(screen.getByText("DairyCo")).toBeInTheDocument();
    const bodies = getBodies();
    expect(bodies.some((b) => b.query.includes("items"))).toBe(true);
    expect(bodies.some((b) => b.query.includes("userItems"))).toBe(true);
    expect(bodies.some((b) => b.query.includes("frequentBrands"))).toBe(true);
  });

  it("creates an item", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Milk"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Butter" } });
    fireEvent.change(screen.getByLabelText("Category ID"), { target: { value: "3" } });
    fireEvent.change(screen.getByLabelText("Unit"), { target: { value: "lb" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createItem"))).toBe(true);
    });
  });

  it("edits an item", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Milk"));
    const row = screen.getByText("Milk").closest("tr")!;
    fireEvent.click(
      row.querySelector('[data-testid="row-edit-button"]')!
    );
    const nameInput = await screen.findByLabelText("Name");
    expect(nameInput).toHaveValue("Milk");
    fireEvent.change(nameInput, { target: { value: "Whole Milk" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateItem") && b.variables.id === "1")).toBe(true);
    });
  });

  it("deletes an item", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Milk"));
    const row = screen.getByText("Milk").closest("tr")!;
    fireEvent.click(
      row.querySelector('[data-testid="row-delete-button"]')!
    );
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteItem") && b.variables.id === "1")).toBe(true);
    });
  });

  it("adds to inventory via the quantity dialog", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Milk"));
    const row = screen.getByText("Milk").closest("tr")!;
    fireEvent.click(
      within(row).getByRole("button", { name: "Add to Inventory" })
    );

    const input = await screen.findByLabelText("How many to add");
    expect(input).toHaveValue("1");

    fireEvent.click(screen.getByRole("button", { name: "Increase quantity" }));
    fireEvent.click(screen.getByRole("button", { name: "Increase quantity" }));
    expect(input).toHaveValue("3");

    fireEvent.change(input, { target: { value: "a4b" } });
    expect(input).toHaveValue("4");

    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) =>
            b.query.includes("adjustUserItem") &&
            b.variables.itemId === "1" &&
            b.variables.quantity === 9
        )
      ).toBe(true);
    });
  });

  it("edits quantity via the column link", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Milk"));
    const row = screen.getByText("Milk").closest("tr")!;
    fireEvent.click(within(row).getByRole("button", { name: "5 gallon" }));

    const input = await screen.findByLabelText("Quantity");
    expect(input).toHaveValue("5");

    fireEvent.change(input, { target: { value: "12" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) =>
            b.query.includes("adjustUserItem") &&
            b.variables.itemId === "1" &&
            b.variables.quantity === 12
        )
      ).toBe(true);
    });
  });

  it("edits favorite via the column link", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Milk"));
    const row = screen.getByText("Milk").closest("tr")!;
    fireEvent.click(within(row).getByRole("button", { name: "No" }));

    const dialog = await screen.findByRole("dialog");
    const toggle = within(dialog).getByRole("switch", { name: "Favorite" });
    expect(toggle).not.toBeChecked();
    fireEvent.click(toggle);
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) =>
            b.query.includes("setItemFavorite") &&
            b.variables.itemId === "1" &&
            b.variables.isFavorite === true
        )
      ).toBe(true);
    });
  });
});
