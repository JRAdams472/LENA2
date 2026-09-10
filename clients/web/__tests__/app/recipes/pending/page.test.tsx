import "@testing-library/jest-dom";
import { render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import PendingRecipesPage from "@/app/recipes/pending/page";

jest.mock("../../../../app/auth/useMe");
import { useMe } from "../../../../app/auth/useMe";

const mockedUseMe = useMe as jest.Mock;

const mockFetch = global.fetch as jest.Mock;

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: jest.fn() })),
  useParams: jest.fn(() => ({})),
  usePathname: jest.fn(() => "/recipes/pending"),
}));

function gql(data: object) {
  return {
    ok: true,
    status: 200,
    headers: { get: () => "application/json" },
    json: async () => ({ data }),
  };
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <PendingRecipesPage />
    </QueryClientProvider>
  );
}

const recipeImport = {
  id: "1",
  sourceFilename: "recipe-scan.png",
  status: "ready",
  ocrText: null,
  draft: null,
  review: null,
  recipe: null,
  profanityFlag: false,
  profanityReason: null,
  errorMessage: null,
};

function meReturn(isAdmin: boolean, isLoading = false) {
  return {
    me: isAdmin ? { userID: 1, role: "admin" } : { userID: 2, role: "member" },
    isAdmin,
    isLoading,
    error: null,
    refetch: jest.fn(),
  };
}

beforeEach(() => {
  mockFetch.mockReset();
});

describe("pending recipes page", () => {
  it("shows Forbidden for non-admins", () => {
    mockedUseMe.mockReturnValue(meReturn(false));
    renderPage();
    expect(screen.getByText(/Forbidden/i)).toBeInTheDocument();
  });

  it("lists pending recipe imports for admins", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation(() =>
      Promise.resolve(
        gql({
          pendingRecipeImports: {
            items: [recipeImport],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      )
    );
    renderPage();
    await waitFor(() =>
      expect(screen.getByText("recipe-scan.png")).toBeInTheDocument()
    );
    expect(screen.getByText("Ready")).toBeInTheDocument();
    const row = screen.getByText("recipe-scan.png").closest("tr")!;
    expect(within(row).getByRole("link", { name: /review/i })).toBeInTheDocument();
  });

  it("renders different status chips", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockImplementation(() =>
      Promise.resolve(
        gql({
          pendingRecipeImports: {
            items: [
              { ...recipeImport, id: "2", status: "reviewing" },
              { ...recipeImport, id: "3", status: "profanity" },
              { ...recipeImport, id: "4", status: "failed" },
              { ...recipeImport, id: "5", status: "ocred" },
              { ...recipeImport, id: "6", status: "unknown" },
            ],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 5 },
          },
        })
      )
    );
    renderPage();
    await waitFor(() =>
      expect(screen.getByText("Reviewing")).toBeInTheDocument()
    );
    expect(screen.getByText("Profanity")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
    expect(screen.getByText("Processing")).toBeInTheDocument();
    expect(screen.getByText("unknown")).toBeInTheDocument();
  });

  it("shows a loading spinner while useMe is loading", () => {
    mockedUseMe.mockReturnValue(meReturn(false, true));
    renderPage();
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });

  it("shows an error if the list fails to load", async () => {
    mockedUseMe.mockReturnValue(meReturn(true));
    mockFetch.mockRejectedValue(new Error("Network down"));
    renderPage();
    await waitFor(() =>
      expect(screen.getByText(/Network down/i)).toBeInTheDocument()
    );
  });
});
