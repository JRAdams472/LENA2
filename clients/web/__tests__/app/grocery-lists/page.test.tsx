import "@testing-library/jest-dom";
import { Suspense } from "react";
import { act, render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import GroceryListsPage from "@/app/grocery-lists/page";
import GroceryListDetailPage from "@/app/grocery-lists/[id]/page";

const mockFetch = global.fetch as jest.Mock;
const mockPush = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: mockPush })),
  useParams: jest.fn(() => ({ id: "1" })),
  usePathname: jest.fn(() => "/grocery-lists"),
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

// Detail pages resolve `params` via React's `use()`, which suspends — the
// render must be awaited inside act() with a Suspense boundary.
async function renderDetailPage(element: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  let result: ReturnType<typeof render> | undefined;
  await act(async () => {
    result = render(
      <QueryClientProvider client={queryClient}>
        <Suspense fallback={null}>{element}</Suspense>
      </QueryClientProvider>
    );
  });
  return result!;
}

beforeEach(() => {
  mockFetch.mockReset();
  mockPush.mockReset();
  jest.spyOn(window, "confirm").mockReturnValue(true);
});

afterEach(() => {
  jest.restoreAllMocks();
});

describe("grocery lists page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("generateGroceryList")) {
        return Promise.resolve(
          gql({ generateGroceryList: { id: "2", generatedAt: "2024-01-02T00:00:00Z", items: [] } })
        );
      }
      return Promise.resolve(
        gql({
          groceryLists: {
            items: [{ id: "1", generatedAt: "2024-01-01T00:00:00Z", items: [] }],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 },
          },
        })
      );
    });
  });

  it("lists grocery lists", async () => {
    renderPage(<GroceryListsPage />);
    await waitFor(() => expect(screen.getByText("2024-01-01")).toBeInTheDocument());
  });

  it("generates a grocery list", async () => {
    renderPage(<GroceryListsPage />);
    await waitFor(() => screen.getByText("2024-01-01"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Meal Plan ID (optional)"), { target: { value: "1" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("generateGroceryList"))).toBe(true);
      expect(mockPush).toHaveBeenCalledWith("/grocery-lists/2");
    });
  });
});

describe("grocery list detail page", () => {
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

  const list = {
    id: "1",
    generatedAt: "2024-01-01T00:00:00Z",
    items: [
      {
        id: "10",
        manualItemName: null,
        quantityNeeded: 2,
        unitOfMeasure: "cup",
        source: "manual",
        isChecked: false,
        item,
      },
    ],
  };

  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("addGroceryItem")) {
        return Promise.resolve(
          gql({
            addGroceryItem: {
              id: "11",
              manualItemName: "Flour",
              quantityNeeded: 3,
              unitOfMeasure: "cup",
              source: "manual",
              isChecked: false,
              item: null,
            },
          })
        );
      }
      if (body.query.includes("toggleGroceryItemChecked")) {
        return Promise.resolve(
          gql({
            toggleGroceryItemChecked: {
              id: "10",
              manualItemName: null,
              quantityNeeded: 2,
              unitOfMeasure: "cup",
              source: "manual",
              isChecked: true,
              item,
            },
          })
        );
      }
      if (body.query.includes("deleteGroceryItem")) {
        return Promise.resolve(gql({ deleteGroceryItem: true }));
      }
      return Promise.resolve(gql({ groceryList: list }));
    });
  });

  it("renders the grocery list", async () => {
    await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
    await waitFor(() => expect(screen.getByText("Grocery List")).toBeInTheDocument());
    expect(screen.getByText(/2024-01-01/)).toBeInTheDocument();
    expect(screen.getByText("Milk")).toBeInTheDocument();
  });

  it("toggles an item checked", async () => {
    await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
    await waitFor(() => screen.getByText("Milk"));
    fireEvent.click(screen.getByRole("checkbox"));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("toggleGroceryItemChecked"))).toBe(true);
    });
  });

  it("adds a manual item", async () => {
    await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
    await waitFor(() => screen.getByText("Milk"));
    fireEvent.change(screen.getByLabelText("Item Name"), { target: { value: "Flour" } });
    fireEvent.change(screen.getByLabelText("Qty"), { target: { value: "3" } });
    fireEvent.change(screen.getByLabelText("Unit"), { target: { value: "cup" } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("addGroceryItem"))).toBe(true);
    });
  });

  it("deletes an item", async () => {
    await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
    await waitFor(() => screen.getByText("Milk"));
    const deleteButton = screen.getByTestId("DeleteIcon").closest("button")!;
    fireEvent.click(deleteButton);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteGroceryItem"))).toBe(true);
    });
  });
});
