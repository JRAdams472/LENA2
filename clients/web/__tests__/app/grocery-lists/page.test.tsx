import "@testing-library/jest-dom";
import { Suspense } from "react";
import { act, render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import GroceryListsPage from "@/app/grocery-lists/page";
import GroceryListDetailPage, { reorderEntries } from "@/app/grocery-lists/[id]/page";
import { GroceryRouteGroup, GroceryListItem } from "@/lib/types";

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
    await waitFor(() =>
      expect(
        screen.getByText(new Date(2024, 0, 1).toLocaleDateString())
      ).toBeInTheDocument()
    );
  });

  it("generates a grocery list", async () => {
    renderPage(<GroceryListsPage />);
    await waitFor(() =>
      screen.getByText(new Date(2024, 0, 1).toLocaleDateString())
    );
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
    store: null,
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

  const routeGroups = [
    {
      aisle: null,
      items: [
        {
          suggested: false,
          item: {
            id: "10",
            manualItemName: null,
            quantityNeeded: 2,
            unitOfMeasure: "cup",
            source: "manual",
            isChecked: false,
            item,
          },
        },
      ],
    },
  ];

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
      if (body.query.includes("groceryRouteGroups")) {
        return Promise.resolve(gql({ groceryRouteGroups: routeGroups }));
      }
      if (body.query.includes("groceryStores")) {
        return Promise.resolve(gql({ groceryStores: [] }));
      }
      if (body.query.includes("suggestedRestockItems")) {
        return Promise.resolve(gql({ suggestedRestockItems: [] }));
      }
      if (body.query.includes("shopperProviders")) {
        return Promise.resolve(gql({ shopperProviders: [] }));
      }
      return Promise.resolve(gql({ groceryList: list }));
    });
  });

  it("renders the grocery list", async () => {
    await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
    await waitFor(() => expect(screen.getByText("Grocery List")).toBeInTheDocument());
    expect(screen.getByText(/Generated/).textContent).toContain(
      new Date(2024, 0, 1).toLocaleDateString()
    );
    expect(screen.getByText("Milk")).toBeInTheDocument();
  });

  it("renders route groups from the server in order", async () => {
    // Route groups are the authoritative order — the page renders them
    // verbatim (no client-side re-grouping by source).
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("groceryRouteGroups")) {
        return Promise.resolve(
          gql({
            groceryRouteGroups: [
              {
                aisle: { id: "5", name: "Produce", position: 0 },
                items: [{ suggested: true, item: routeGroups[0].items[0].item }],
              },
              {
                aisle: null,
                items: [],
              },
            ],
          })
        );
      }
      if (body.query.includes("groceryStores")) {
        return Promise.resolve(gql({ groceryStores: [] }));
      }
      if (body.query.includes("suggestedRestockItems")) {
        return Promise.resolve(gql({ suggestedRestockItems: [] }));
      }
      if (body.query.includes("shopperProviders")) {
        return Promise.resolve(gql({ shopperProviders: [] }));
      }
      return Promise.resolve(gql({ groceryList: list }));
    });
    await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
    await waitFor(() => expect(screen.getByText("Produce")).toBeInTheDocument());
    expect(screen.getByText("Milk")).toBeInTheDocument();
    expect(screen.getByText("suggested aisle")).toBeInTheDocument();
  });

  it("shows the store picker and changes the list's store", async () => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("groceryRouteGroups")) {
        return Promise.resolve(gql({ groceryRouteGroups: routeGroups }));
      }
      if (body.query.includes("groceryStores")) {
        return Promise.resolve(
          gql({ groceryStores: [{ id: "7", name: "Costco", aisles: [] }] })
        );
      }
      if (body.query.includes("setGroceryListStore")) {
        return Promise.resolve(gql({ setGroceryListStore: { ...list, store: { id: "7", name: "Costco" } } }));
      }
      if (body.query.includes("suggestedRestockItems")) {
        return Promise.resolve(gql({ suggestedRestockItems: [] }));
      }
      if (body.query.includes("shopperProviders")) {
        return Promise.resolve(gql({ shopperProviders: [] }));
      }
      return Promise.resolve(gql({ groceryList: list }));
    });
    await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
    await waitFor(() => screen.getByText("Milk"));
    fireEvent.mouseDown(screen.getByLabelText("Store"));
    await waitFor(() => screen.getByText("Costco"));
    fireEvent.click(screen.getByText("Costco"));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("setGroceryListStore"))).toBe(true);
    });
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
    fireEvent.change(screen.getByLabelText("Ingredient or item"), { target: { value: "Flour" } });
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

  describe("shop with Instacart", () => {
    const link = {
      provider: "INSTACART",
      url: "https://instacart.example.com/list/abc123",
    };

    function mockShop(opts: {
      providers?: string[];
      linkError?: string;
      listOverride?: object;
    } = {}) {
      const providers = opts.providers ?? ["INSTACART"];
      const theList = opts.listOverride ?? list;
      mockFetch.mockImplementation((_, init) => {
        const body = JSON.parse((init as RequestInit).body as string);
        if (body.query.includes("createShoppingLink")) {
          if (opts.linkError) {
            return Promise.resolve({
              ok: true,
              status: 200,
              headers: { get: () => "application/json" },
              json: async () => ({ errors: [{ message: opts.linkError }] }),
            });
          }
          return Promise.resolve(gql({ createShoppingLink: link }));
        }
        if (body.query.includes("shopperProviders")) {
          return Promise.resolve(gql({ shopperProviders: providers }));
        }
        if (body.query.includes("groceryRouteGroups")) {
          return Promise.resolve(gql({ groceryRouteGroups: routeGroups }));
        }
        if (body.query.includes("groceryStores")) {
          return Promise.resolve(gql({ groceryStores: [] }));
        }
        if (body.query.includes("suggestedRestockItems")) {
          return Promise.resolve(gql({ suggestedRestockItems: [] }));
        }
        return Promise.resolve(gql({ groceryList: theList }));
      });
    }

    it("hides the action when no provider is configured", async () => {
      mockShop({ providers: [] });
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Milk"));
      expect(
        screen.queryByRole("button", { name: "Shop with Instacart" })
      ).not.toBeInTheDocument();
    });

    it("creates a link and shows it in a dialog", async () => {
      mockShop();
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Milk"));

      fireEvent.click(screen.getByRole("button", { name: "Shop with Instacart" }));
      await waitFor(() =>
        expect(getBodies().some((b) => b.query.includes("createShoppingLink"))).toBe(true)
      );

      await screen.findByRole("dialog");
      expect(
        screen.getByDisplayValue("https://instacart.example.com/list/abc123")
      ).toBeInTheDocument();
    });

    it("notes when checked items were left off the link", async () => {
      const checkedList = {
        ...list,
        items: [{ ...list.items[0], isChecked: true }],
      };
      mockShop({ listOverride: checkedList });
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Milk"));

      fireEvent.click(screen.getByRole("button", { name: "Shop with Instacart" }));
      await waitFor(() =>
        expect(
          screen.getByText(/checked items were left off/i)
        ).toBeInTheDocument()
      );
    });

    it("opens the link in a new tab and copies it", async () => {
      const openSpy = jest.spyOn(window, "open").mockImplementation(() => null);
      const writeText = jest.fn().mockResolvedValue(undefined);
      Object.assign(navigator, { clipboard: { writeText } });
      mockShop();
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Milk"));

      fireEvent.click(screen.getByRole("button", { name: "Shop with Instacart" }));
      await screen.findByRole("dialog");

      fireEvent.click(screen.getByRole("button", { name: /open instacart/i }));
      expect(openSpy).toHaveBeenCalledWith(
        "https://instacart.example.com/list/abc123",
        "_blank",
        "noopener,noreferrer"
      );

      fireEvent.click(screen.getByRole("button", { name: /copy link/i }));
      await waitFor(() =>
        expect(writeText).toHaveBeenCalledWith("https://instacart.example.com/list/abc123")
      );
      await waitFor(() =>
        expect(screen.getByRole("button", { name: "Copied" })).toBeInTheDocument()
      );
    });

    it("shows the error when the mutation fails", async () => {
      mockShop({ linkError: "shopping provider unavailable" });
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Milk"));

      fireEvent.click(screen.getByRole("button", { name: "Shop with Instacart" }));
      await waitFor(() =>
        expect(screen.getByText(/shopping provider unavailable/)).toBeInTheDocument()
      );
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  describe("store aisles and restock", () => {
    const aisles = [
      { id: "1", name: "Produce", position: 0 },
      { id: "2", name: "Dairy", position: 1 },
    ];
    const store = { id: "7", name: "Costco", aisles };
    const storedList = { ...list, store };
    // The list's item sits inside the Produce aisle group so the move and
    // unassign paths both see a real target/source aisle.
    const groupedRoutes = [
      { aisle: aisles[0], items: [routeGroups[0].items[0]] },
      { aisle: null, items: [] },
    ];

    function mockStore(opts: { restock?: object[] } = {}) {
      mockFetch.mockImplementation((_, init) => {
        const body = JSON.parse((init as RequestInit).body as string);
        if (body.query.includes("assignItemToAisle")) {
          return Promise.resolve(gql({ assignItemToAisle: true }));
        }
        if (body.query.includes("reorderGroceryListItems")) {
          return Promise.resolve(gql({ reorderGroceryListItems: true }));
        }
        if (body.query.includes("createStoreAisle")) {
          return Promise.resolve(
            gql({ createStoreAisle: { id: "9", name: "Bakery", position: 2 } })
          );
        }
        if (body.query.includes("renameStoreAisle")) {
          return Promise.resolve(
            gql({ renameStoreAisle: { id: "1", name: "Fresh", position: 0 } })
          );
        }
        if (body.query.includes("deleteStoreAisle")) {
          return Promise.resolve(gql({ deleteStoreAisle: true }));
        }
        if (body.query.includes("reorderStoreAisles")) {
          return Promise.resolve(gql({ reorderStoreAisles: aisles }));
        }
        if (body.query.includes("addGroceryItem")) {
          return Promise.resolve(
            gql({
              addGroceryItem: {
                id: "11",
                manualItemName: null,
                quantityNeeded: 1,
                unitOfMeasure: "",
                source: "pantry",
                isChecked: false,
                item: null,
              },
            })
          );
        }
        if (body.query.includes("groceryRouteGroups")) {
          return Promise.resolve(gql({ groceryRouteGroups: groupedRoutes }));
        }
        if (body.query.includes("groceryStores")) {
          return Promise.resolve(gql({ groceryStores: [store] }));
        }
        if (body.query.includes("suggestedRestockItems")) {
          return Promise.resolve(
            gql({ suggestedRestockItems: opts.restock ?? [] })
          );
        }
        if (body.query.includes("shopperProviders")) {
          return Promise.resolve(gql({ shopperProviders: [] }));
        }
        return Promise.resolve(gql({ groceryList: storedList }));
      });
    }

    it("moves an item to unassigned, clearing its aisle", async () => {
      mockStore();
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Milk"));

      fireEvent.click(screen.getByLabelText("item actions"));
      fireEvent.click(await screen.findByText("Move to unassigned"));
      await waitFor(() => {
        const bodies = getBodies();
        expect(bodies.some((b) => b.query.includes("assignItemToAisle"))).toBe(true);
        expect(
          bodies.some((b) => b.query.includes("reorderGroceryListItems"))
        ).toBe(true);
      });
    });

    it("moves an item to another aisle", async () => {
      mockStore();
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Milk"));

      fireEvent.click(screen.getByLabelText("item actions"));
      fireEvent.click(await screen.findByText("Move to Dairy"));
      await waitFor(() => {
        expect(
          getBodies().some((b) => b.query.includes("reorderGroceryListItems"))
        ).toBe(true);
      });
    });

    it("manages the store's aisle layout", async () => {
      mockStore();
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Milk"));

      fireEvent.click(screen.getByRole("button", { name: "Edit aisles" }));
      await screen.findByText(/Costco — aisle layout/);

      const upButtons = screen.getAllByLabelText("move aisle up");
      const downButtons = screen.getAllByLabelText("move aisle down");
      expect(upButtons[0]).toBeDisabled();
      expect(downButtons[downButtons.length - 1]).toBeDisabled();

      fireEvent.click(downButtons[0]);
      await waitFor(() =>
        expect(getBodies().some((b) => b.query.includes("reorderStoreAisles"))).toBe(true)
      );

      fireEvent.change(screen.getByDisplayValue("Produce"), { target: { value: "Fresh" } });
      fireEvent.blur(screen.getByDisplayValue("Fresh"));
      await waitFor(() =>
        expect(getBodies().some((b) => b.query.includes("renameStoreAisle"))).toBe(true)
      );

      fireEvent.click(screen.getAllByLabelText("delete aisle")[0]);
      await waitFor(() =>
        expect(getBodies().some((b) => b.query.includes("deleteStoreAisle"))).toBe(true)
      );

      fireEvent.change(screen.getByLabelText("New aisle"), { target: { value: "Bakery" } });
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
      await waitFor(() =>
        expect(getBodies().some((b) => b.query.includes("createStoreAisle"))).toBe(true)
      );

      fireEvent.click(screen.getByRole("button", { name: "Done" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    });

    it("lists restock suggestions and adds one", async () => {
      mockStore({
        restock: [
          {
            id: "9",
            name: "Olive Oil 500ml",
            upc12: null,
            upc14: null,
            unit: "ml",
            brand: { id: "8", name: "Pure" },
            category: null,
            nutrients: [],
            flavors: [],
          },
        ],
      });
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Time to restock"));

      const addButtons = screen.getAllByRole("button", { name: "Add" });
      fireEvent.click(addButtons[addButtons.length - 1]);
      await waitFor(() =>
        expect(getBodies().some((b) => b.query.includes("addGroceryItem"))).toBe(true)
      );
    });

    it("asks for a brand when checking an ingredient-only line", async () => {
      const ingredientOnly = {
        id: "12",
        manualItemName: null,
        quantityNeeded: 1,
        unitOfMeasure: "",
        source: "meal-plan",
        isChecked: false,
        item: null,
        ingredient: { id: "4", name: "Tomato" },
        usualBrand: null,
      };
      mockFetch.mockImplementation((_, init) => {
        const body = JSON.parse((init as RequestInit).body as string);
        if (body.query.includes("toggleGroceryItemChecked")) {
          return Promise.resolve(
            gql({ toggleGroceryItemChecked: { ...ingredientOnly, isChecked: true } })
          );
        }
        if (body.query.includes("groceryRouteGroups")) {
          return Promise.resolve(
            gql({
              groceryRouteGroups: [
                { aisle: null, items: [{ suggested: false, item: ingredientOnly }] },
              ],
            })
          );
        }
        if (body.query.includes("groceryStores")) {
          return Promise.resolve(gql({ groceryStores: [] }));
        }
        if (body.query.includes("suggestedRestockItems")) {
          return Promise.resolve(gql({ suggestedRestockItems: [] }));
        }
        if (body.query.includes("shopperProviders")) {
          return Promise.resolve(gql({ shopperProviders: [] }));
        }
        return Promise.resolve(
          gql({ groceryList: { ...list, items: [ingredientOnly] } })
        );
      });
      await renderDetailPage(<GroceryListDetailPage params={Promise.resolve({ id: "1" })} />);
      await waitFor(() => screen.getByText("Tomato"));

      fireEvent.click(screen.getByRole("checkbox"));
      await screen.findByText(/Which Tomato did you buy/);
      // Toggling must NOT fire yet — the dialog intercepts it.
      expect(
        getBodies().some((b) => b.query.includes("toggleGroceryItemChecked"))
      ).toBe(false);

      fireEvent.click(screen.getByRole("button", { name: "Check off without a brand" }));
      await waitFor(() =>
        expect(
          getBodies().some((b) => b.query.includes("toggleGroceryItemChecked"))
        ).toBe(true)
      );
    });
  });
});

describe("reorderEntries", () => {
  const mkItem = (id: number): GroceryListItem => ({
    groceryListItemID: id,
    groceryListID: 1,
    itemID: null,
    itemName: null,
    manualItemName: `Item ${id}`,
    quantityNeeded: 1,
    unitOfMeasure: null,
    source: "manual",
    isChecked: false,
    createdBy: "t",
    createDate: "2024-01-01",
    lastUpdatedBy: null,
    lastUpdatedDate: null,
  });
  const mkGroup = (aisleID: number | null, ids: number[]): GroceryRouteGroup => ({
    aisle: aisleID != null ? { aisleID, name: `Aisle ${aisleID}`, position: aisleID } : null,
    items: ids.map((id) => ({ suggested: false, item: mkItem(id) })),
  });

  it("moves an item to another group carrying that aisle", () => {
    const groups = [mkGroup(1, [10, 11]), mkGroup(2, [20]), mkGroup(null, [30])];
    const entries = reorderEntries(groups, 10, 1, 1); // drop item 10 at end of aisle 2
    expect(entries.map((e) => e.groceryListItemID)).toEqual([11, 20, 10, 30]);
    expect(entries.find((e) => e.groceryListItemID === 10)?.aisleID).toBe(2);
    // Other items carry no aisle change.
    expect(entries.find((e) => e.groceryListItemID === 20)?.aisleID).toBeUndefined();
  });

  it("reorders within a group without aisle change", () => {
    const groups = [mkGroup(1, [10, 11, 12])];
    const entries = reorderEntries(groups, 10, 0, 2); // item 10 after 12
    expect(entries.map((e) => e.groceryListItemID)).toEqual([11, 12, 10]);
    expect(entries[2].aisleID).toBe(1);
  });

  it("drops into the unassigned bucket at a position", () => {
    const groups = [mkGroup(1, [10]), mkGroup(null, [30, 31])];
    const entries = reorderEntries(groups, 10, 1, 1); // item 10 between 30 and 31
    expect(entries.map((e) => e.groceryListItemID)).toEqual([30, 10, 31]);
    // Unassigned bucket has no aisle — undefined = no change signal.
    expect(entries.find((e) => e.groceryListItemID === 10)?.aisleID).toBeUndefined();
  });
});
