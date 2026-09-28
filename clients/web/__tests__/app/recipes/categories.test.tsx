import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import RecipeCategoriesPage from "@/app/recipes/categories/page";

jest.mock("../../../app/auth/useMe");
import { useMe } from "../../../app/auth/useMe";

const mockedUseMe = useMe as jest.Mock;
const mockFetch = global.fetch as jest.Mock;

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: jest.fn() })),
  usePathname: jest.fn(() => "/recipes/categories"),
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

const groups = {
  recipeCategoryGroups: [
    {
      id: "3",
      name: "Cuisine",
      exclusive: true,
      displayOrder: 6,
      categories: [
        { id: "21", name: "Mexican", group: { id: "3", name: "Cuisine", exclusive: true, displayOrder: 6 } },
        { id: "22", name: "Italian", group: { id: "3", name: "Cuisine", exclusive: true, displayOrder: 6 } },
      ],
    },
    {
      id: "2",
      name: "Dish Type",
      exclusive: false,
      displayOrder: 2,
      categories: [
        { id: "40", name: "Soup", group: { id: "2", name: "Dish Type", exclusive: false, displayOrder: 2 } },
      ],
    },
  ],
};

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <RecipeCategoriesPage />
    </QueryClientProvider>
  );
}

beforeEach(() => {
  mockFetch.mockReset();
  mockedUseMe.mockReturnValue({ me: { role: "admin" }, isAdmin: true, isLoading: false });
  mockFetch.mockImplementation((_, init) => {
    const body = JSON.parse((init as RequestInit).body as string);
    if (body.query.includes("createRecipeCategoryGroup")) {
      return Promise.resolve(
        gql({ createRecipeCategoryGroup: { id: "9", name: "Season", exclusive: true, displayOrder: 7 } })
      );
    }
    if (body.query.includes("updateRecipeCategoryGroup")) {
      return Promise.resolve(
        gql({ updateRecipeCategoryGroup: { id: "3", name: "Cuisines", exclusive: true, displayOrder: 6 } })
      );
    }
    if (body.query.includes("deleteRecipeCategoryGroup")) {
      return Promise.resolve(gql({ deleteRecipeCategoryGroup: true }));
    }
    if (body.query.includes("createRecipeCategory")) {
      return Promise.resolve(
        gql({ createRecipeCategory: { id: "30", name: "Thai", group: { id: "3", name: "Cuisine", exclusive: true, displayOrder: 6 } } })
      );
    }
    if (body.query.includes("updateRecipeCategory")) {
      return Promise.resolve(
        gql({ updateRecipeCategory: { id: "21", name: "Tex-Mex", group: { id: "3", name: "Cuisine", exclusive: true, displayOrder: 6 } } })
      );
    }
    if (body.query.includes("deleteRecipeCategory")) {
      return Promise.resolve(gql({ deleteRecipeCategory: true }));
    }
    return Promise.resolve(gql(groups));
  });
});

describe("recipe categories admin page", () => {
  it("lists groups with exclusivity and counts", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Cuisine")).toBeInTheDocument());
    expect(screen.getByText("Dish Type")).toBeInTheDocument();
    expect(screen.getAllByText("Yes").length).toBeGreaterThan(0);
    expect(screen.getAllByText("No").length).toBeGreaterThan(0);
  });

  it("creates a group", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Cuisine"));
    fireEvent.click(screen.getByRole("button", { name: /new group/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Season" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) => b.query.includes("createRecipeCategoryGroup") &&
            b.variables?.input?.name === "Season"
        )
      ).toBe(true);
    });
  });

  it("edits a group", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Cuisine"));
    fireEvent.click(screen.getByLabelText("edit Cuisine"));
    const nameField = await screen.findByLabelText("Name");
    fireEvent.change(nameField, { target: { value: "Cuisines" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) => b.query.includes("updateRecipeCategoryGroup") &&
            b.variables?.id === "3" &&
            b.variables?.input?.name === "Cuisines"
        )
      ).toBe(true);
    });
  });

  it("deletes a group", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Cuisine"));
    fireEvent.click(screen.getByLabelText("delete Cuisine"));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) => b.query.includes("deleteRecipeCategoryGroup") && b.variables?.id === "3"
        )
      ).toBe(true);
    });
  });

  it("shows a group's categories when selected", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Cuisine"));
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(await screen.findByRole("option", { name: "Cuisine" }));
    await waitFor(() => {
      expect(screen.getByText("Mexican")).toBeInTheDocument();
      expect(screen.getByText("Italian")).toBeInTheDocument();
    });
  });

  it("creates a category in the selected group", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Cuisine"));
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(await screen.findByRole("option", { name: "Cuisine" }));
    fireEvent.click(await screen.findByRole("button", { name: /new category/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Thai" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) => b.query.includes("createRecipeCategory") &&
            b.variables?.input?.groupId === "3" &&
            b.variables?.input?.name === "Thai"
        )
      ).toBe(true);
    });
  });

  it("renames a category", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Cuisine"));
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(await screen.findByRole("option", { name: "Cuisine" }));
    fireEvent.click(await screen.findByLabelText("edit Mexican"));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Tex-Mex" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) => b.query.includes("updateRecipeCategory") &&
            b.variables?.id === "21" &&
            b.variables?.input?.name === "Tex-Mex"
        )
      ).toBe(true);
    });
  });

  it("deletes a category", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Cuisine"));
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(await screen.findByRole("option", { name: "Cuisine" }));
    fireEvent.click(await screen.findByLabelText("delete Mexican"));
    await waitFor(() => {
      expect(
        getBodies().some(
          (b) => b.query.includes("deleteRecipeCategory") && b.variables?.id === "21"
        )
      ).toBe(true);
    });
  });

  it("hides edit controls for non-admins", async () => {
    mockedUseMe.mockReturnValue({ me: { role: "member" }, isAdmin: false, isLoading: false });
    renderPage();
    await waitFor(() => screen.getByText("Cuisine"));
    expect(screen.getByText(/only admins/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /new group/i })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("edit Cuisine")).not.toBeInTheDocument();
  });
});
