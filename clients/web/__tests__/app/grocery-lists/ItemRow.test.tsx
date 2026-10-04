import "@testing-library/jest-dom";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ItemRow } from "@/app/grocery-lists/[id]/page";
import { GroceryListItem } from "@/lib/types";

const mockFetch = global.fetch as jest.Mock;

function gql(data: object) {
  return {
    ok: true,
    status: 200,
    headers: { get: () => "application/json" },
    json: async () => ({ data }),
  };
}

const queryClient = new QueryClient();

function Wrapper({ children }: { children: React.ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

const baseGroceryItem = (overrides: Partial<GroceryListItem> = {}): GroceryListItem => ({
  groceryListItemID: 1,
  groceryListID: 1,
  itemID: null,
  itemName: null,
  manualItemName: null,
  quantityNeeded: 1,
  unitOfMeasure: "unit",
  source: "Manual",
  isChecked: false,
  createdBy: "test",
  createDate: "2024-01-01T00:00:00Z",
  lastUpdatedBy: null,
  lastUpdatedDate: null,
  ...overrides,
});

describe("ItemRow", () => {
  it("uses manualItemName when present", () => {
    const item = baseGroceryItem({
      itemID: null,
      itemName: null,
      manualItemName: "Custom Manual Entry",
    });

    render(<ItemRow item={item} listId={1} />, { wrapper: Wrapper });

    expect(screen.getByText("Custom Manual Entry")).toBeInTheDocument();
  });

  it("uses itemName when present", () => {
    const item = baseGroceryItem({
      itemID: 1,
      itemName: "Whole Milk",
      manualItemName: null,
    });

    render(<ItemRow item={item} listId={1} />, { wrapper: Wrapper });

    expect(screen.getByText("Whole Milk")).toBeInTheDocument();
  });

  it("falls back to 'Item {itemID}' when no name is found", () => {
    const item = baseGroceryItem({
      itemID: 99,
      itemName: null,
      manualItemName: null,
    });

    render(<ItemRow item={item} listId={1} />, { wrapper: Wrapper });

    expect(screen.getByText("Item 99")).toBeInTheDocument();
  });

  it("shows the ingredient name as the primary label", () => {
    const item = baseGroceryItem({
      ingredientID: 7,
      ingredientName: "corn",
      usualBrandName: "Green Giant Whole Kernel Corn",
    });

    render(<ItemRow item={item} listId={1} />, { wrapper: Wrapper });

    expect(screen.getByText("corn")).toBeInTheDocument();
    expect(screen.getByText(/usual: Green Giant Whole Kernel Corn/)).toBeInTheDocument();
  });
});

describe("ItemRow brand-picked check-off", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("checkGroceryItemWithBrand")) {
        return Promise.resolve(
          gql({
            checkGroceryItemWithBrand: {
              id: "1",
              isChecked: true,
              manualItemName: null,
              quantityNeeded: 1,
              unitOfMeasure: "can",
              source: "meal_plan",
              ingredient: { id: "7", name: "corn" },
              usualBrand: { id: "42", name: "Whole Kernel", brand: { id: "1", name: "Green Giant" } },
              item: null,
            },
          })
        );
      }
      if (body.query.includes("toggleGroceryItemChecked")) {
        return Promise.resolve(
          gql({
            toggleGroceryItemChecked: {
              id: "1",
              isChecked: true,
              manualItemName: null,
              quantityNeeded: 1,
              unitOfMeasure: "can",
              source: "meal_plan",
              item: null,
            },
          })
        );
      }
      if (body.query.includes("items(")) {
        return Promise.resolve(
          gql({
            items: {
              items: [
                {
                  id: "42",
                  name: "Whole Kernel Corn",
                  brand: { id: "1", name: "Green Giant" },
                  upc12: null,
                  upc14: null,
                  unit: "can",
                  category: { id: "3", name: "Vegetables", description: null },
                  ingredient: { id: "7", name: "corn" },
                  householdIngredient: { id: "7", name: "corn" },
                  nutrients: [],
                  flavors: [],
                  status: "approved",
                  submittedByMe: false,
                  selectionCount: 0,
                  personalSelectionCount: 0,
                },
              ],
            },
          })
        );
      }
      return Promise.resolve(gql({}));
    });
  });

  const ingredientOnlyRow = () =>
    baseGroceryItem({
      groceryListItemID: 1,
      ingredientID: 7,
      ingredientName: "corn",
      unitOfMeasure: "can",
      source: "meal_plan",
    });

  it("opens the brand picker for an ingredient-only line, then checks off with the brand", async () => {
    render(<ItemRow item={ingredientOnlyRow()} listId={1} />, { wrapper: Wrapper });

    fireEvent.click(screen.getByRole("checkbox"));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Which corn did you buy?");

    const picker = screen.getByLabelText("Brand or item");
    fireEvent.change(picker, { target: { value: "whole" } });
    const option = await screen.findByRole("option", { name: /Green Giant Whole Kernel Corn/ });
    fireEvent.click(option);

    await waitFor(() => {
      const calls = mockFetch.mock.calls.map((c) =>
        JSON.parse((c[1] as RequestInit).body as string)
      );
      const brandCheck = calls.find((b) => b.query.includes("checkGroceryItemWithBrand"));
      expect(brandCheck).toBeTruthy();
      expect(brandCheck.variables).toEqual({ groceryListItemId: "1", itemId: "42" });
    });
  });

  it("skips the brand picker for a manual line and toggles normally", async () => {
    render(
      <ItemRow
        item={baseGroceryItem({ manualItemName: "Paper towels" })}
        listId={1}
      />,
      { wrapper: Wrapper }
    );

    fireEvent.click(screen.getByRole("checkbox"));

    await waitFor(() => {
      const calls = mockFetch.mock.calls.map((c) =>
        JSON.parse((c[1] as RequestInit).body as string)
      );
      expect(calls.find((b) => b.query.includes("toggleGroceryItemChecked"))).toBeTruthy();
      expect(calls.find((b) => b.query.includes("checkGroceryItemWithBrand"))).toBeUndefined();
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("shows an allergy warning chip when the line conflicts", () => {
    const item = baseGroceryItem({
      manualItemName: "Peanut butter",
      allergyWarnings: [
        {
          member: { userID: 8, displayName: "Ada", firstName: null, lastName: null },
          allergen: { allergenID: 1, name: "Peanuts", description: null, isActive: true },
          memberKind: "allergy",
          entityKind: "contains",
        },
      ],
    });

    render(<ItemRow item={item} listId={1} />, { wrapper: Wrapper });

    expect(screen.getByTestId("allergy-warning-chip")).toHaveTextContent(
      "1 allergy warning"
    );
  });

  it("renders no chip when the line has no warnings", () => {
    render(
      <ItemRow item={baseGroceryItem({ manualItemName: "Apples" })} listId={1} />,
      { wrapper: Wrapper }
    );

    expect(screen.queryByTestId("allergy-warning-chip")).not.toBeInTheDocument();
  });
});
