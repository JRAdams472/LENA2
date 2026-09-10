import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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
    fireEvent.click(screen.getByRole("button", { name: /reject/i }));
    await waitFor(() =>
      expect(getBodies().some((b) => b.query.includes("rejectRecipeImport"))).toBe(true)
    );
    expect(mockPush).toHaveBeenCalledWith("/recipes/pending");
  });

  it("retries the import", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation((_, init) => {
      const body = init ? JSON.parse((init as RequestInit).body as string) : { query: "" };
      if (body.query?.includes("retryRecipeImport")) {
        return Promise.resolve(gql({ retryRecipeImport: { ...recipeImport, status: "pending" } }));
      }
      return Promise.resolve(gql({ recipeImport }));
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
});
