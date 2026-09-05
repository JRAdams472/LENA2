import "@testing-library/jest-dom";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ItemRow } from "@/app/grocery-lists/[id]/page";
import { GroceryListItem } from "@/lib/types";

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
});
