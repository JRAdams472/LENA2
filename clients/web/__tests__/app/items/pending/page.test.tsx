import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import PendingItemsPage from "@/app/items/pending/page";
import { api, ApiError } from "@/lib/api";
import { Item } from "@/lib/types";
jest.mock("../../../../app/auth/useMe");
import { useMe } from "@/app/auth/useMe";

jest.mock("../../../../lib/api", () => ({
  api: {
    getPendingItems: jest.fn(),
    getItemByUpc: jest.fn(),
    approveItem: jest.fn(),
    rejectItem: jest.fn(),
  },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

const mockedUseMe = useMe as jest.Mock;
const mockedApi = api as jest.Mocked<typeof api>;

const brand = { brandID: 1, brandName: "Test Brand", selectionCount: 0, personalSelectionCount: 0 };
const category = {
  categoryID: 1,
  categoryName: "Dairy",
  description: null,
  isActive: true,
  isProtein: false,
  createdBy: "system",
  createDate: "2025-01-01T00:00:00Z",
  lastUpdatedBy: null,
  lastUpdatedDate: null,
};

const baseItem: Item = {
  itemID: 1,
  name: "Pending Milk",
  brand: brand.brandName,
  upc12: "123456789012",
  upc14: null,
  categoryID: category.categoryID,
  unit: "gallon",
  currentQuantity: 0,
  minQuantity: null,
  purchaseDate: null,
  expiryDate: null,
  notes: null,
  isFavorite: false,
  status: "pending",
  submittedByMe: false,
  category,
  foodNutrients: [],
  foodFlavors: [],
  selectionCount: 0,
  personalSelectionCount: 0,
  createdBy: "user",
  createDate: "2025-01-01T00:00:00Z",
  lastUpdatedBy: null,
  lastUpdatedDate: null,
};

function meReturn(isAdmin: boolean) {
  return { me: isAdmin ? { userID: 1, role: "admin" } : { userID: 2, role: "member" }, isAdmin, isLoading: false, error: null, refetch: jest.fn() };
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <PendingItemsPage />
    </QueryClientProvider>
  );
}

describe("pending items admin page", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("shows Forbidden for non-admins", () => {
    mockedUseMe.mockReturnValue(meReturn(false));
    renderPage();
    expect(screen.getByText(/Forbidden/i)).toBeInTheDocument();
    expect(mockedApi.getPendingItems).not.toHaveBeenCalled();
  });

  it("lists pending items for admins", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockedApi.getPendingItems.mockResolvedValue({
      items: [baseItem],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 1,
      totalPages: 1,
    });

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("Pending Milk")).toBeInTheDocument()
    );
    expect(screen.getByText("Test Brand")).toBeInTheDocument();
  });

  it("shows approve and reject buttons", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockedApi.getPendingItems.mockResolvedValue({
      items: [baseItem],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 1,
      totalPages: 1,
    });

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("Pending Milk")).toBeInTheDocument()
    );

    const rows = screen.getAllByRole("row").slice(1);
    expect(rows).toHaveLength(1);
    expect(within(rows[0]).getByRole("button", { name: /approve/i })).toBeInTheDocument();
    expect(within(rows[0]).getByRole("button", { name: /reject/i })).toBeInTheDocument();
  });

  it("approves an item through the confirm dialog", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockedApi.getPendingItems.mockResolvedValue({
      items: [{ ...baseItem, submittedByMe: true }],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 1,
      totalPages: 1,
    });
    mockedApi.approveItem.mockResolvedValue({ ...baseItem, status: "approved" });

    renderPage();
    await waitFor(() => expect(screen.getByText("Pending Milk")).toBeInTheDocument());
    expect(screen.getByText("You")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    expect(screen.getByText(/will become visible to all users/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /confirm/i }));

    await waitFor(() => expect(mockedApi.approveItem).toHaveBeenCalledWith(1));
  });

  it("rejects an item through the confirm dialog", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockedApi.getPendingItems.mockResolvedValue({
      items: [baseItem],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 1,
      totalPages: 1,
    });
    mockedApi.rejectItem.mockResolvedValue({ ...baseItem, status: "rejected" });

    renderPage();
    await waitFor(() => expect(screen.getByText("Pending Milk")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /reject/i }));
    expect(screen.getByText(/will be marked rejected/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /confirm/i }));

    await waitFor(() => expect(mockedApi.rejectItem).toHaveBeenCalledWith(1));
  });

  it("cancels the confirm dialog", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockedApi.getPendingItems.mockResolvedValue({
      items: [baseItem],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 1,
      totalPages: 1,
    });

    renderPage();
    await waitFor(() => expect(screen.getByText("Pending Milk")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    await waitFor(() =>
      expect(screen.queryByText(/will become visible/)).not.toBeInTheDocument()
    );
    expect(mockedApi.approveItem).not.toHaveBeenCalled();
  });

  it("shows the api error when approve fails", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockedApi.getPendingItems.mockResolvedValue({
      items: [baseItem],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 1,
      totalPages: 1,
    });
    mockedApi.approveItem.mockRejectedValue(new ApiError(409, "already approved"));

    renderPage();
    await waitFor(() => expect(screen.getByText("Pending Milk")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    fireEvent.click(screen.getByRole("button", { name: /confirm/i }));

    await waitFor(() => expect(screen.getByText("already approved")).toBeInTheDocument());
  });

  it("renders nutrient summary and upc fallbacks", async () => {
    const withNutrients: Item = {
      ...baseItem,
      upc12: null,
      upc14: "12345678901234",
      foodNutrients: [
        { nutrientType: { nutrientName: "Calories" }, amountPerServing: 120 },
        { nutrientType: { nutrientName: "Protein" }, amountPerServing: 8 },
        { nutrientType: { nutrientName: "Fat" }, amountPerServing: 5 },
        { nutrientType: { nutrientName: "Carbs" }, amountPerServing: 12 },
      ] as Item["foodNutrients"],
    };
    mockedUseMe.mockReturnValue(meReturn(true));
    mockedApi.getPendingItems.mockResolvedValue({
      items: [withNutrients],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 1,
      totalPages: 1,
    });

    renderPage();
    await waitFor(() =>
      expect(
        screen.getByText(/Calories: 120, Protein: 8, Fat: 5 \(\+1 more\)/)
      ).toBeInTheDocument()
    );
    expect(screen.getByText("12345678901234")).toBeInTheDocument();
  });

  it("paginates through the pending list", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockedApi.getPendingItems.mockResolvedValue({
      items: [baseItem],
      pageNumber: 1,
      pageSize: 25,
      totalCount: 50,
      totalPages: 2,
    });

    renderPage();
    await waitFor(() => expect(screen.getByText("Pending Milk")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() =>
      expect(mockedApi.getPendingItems).toHaveBeenCalledWith(2, 25)
    );
  });
});
