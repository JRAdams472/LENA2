import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import BrandsPage from "@/app/inventory/brands/page";
import FlavorProfilesPage from "@/app/inventory/flavor-profiles/page";
import FoodFlavorsPage from "@/app/inventory/food-flavors/page";
import FoodNutrientsPage from "@/app/inventory/food-nutrients/page";
import NutrientTypesPage from "@/app/inventory/nutrient-types/page";

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

function renderPage(element: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>{element}</QueryClientProvider>
  );
}

beforeEach(() => {
  mockFetch.mockReset();
  jest.spyOn(window, "confirm").mockReturnValue(true);
});

afterEach(() => {
  jest.restoreAllMocks();
});

describe("brands page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createBrand")) {
        return Promise.resolve(gql({ createBrand: { id: "2", name: "Adidas" } }));
      }
      if (body.query.includes("updateBrand")) {
        return Promise.resolve(gql({ updateBrand: { id: "1", name: "Nike USA" } }));
      }
      if (body.query.includes("deleteBrand")) {
        return Promise.resolve(gql({ deleteBrand: true }));
      }
      return Promise.resolve(gql({ brands: [{ id: "1", name: "Nike" }] }));
    });
  });

  it("lists brands", async () => {
    renderPage(<BrandsPage />);
    await waitFor(() => expect(screen.getByText("Nike")).toBeInTheDocument());
    const bodies = getBodies();
    expect(bodies.some((b) => b.query.includes("brands"))).toBe(true);
  });

  it("creates a brand", async () => {
    renderPage(<BrandsPage />);
    await waitFor(() => screen.getByText("Nike"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Brand Name"), { target: { value: "Adidas" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createBrand"))).toBe(true);
    });
  });

  it("edits a brand", async () => {
    renderPage(<BrandsPage />);
    await waitFor(() => screen.getByText("Nike"));
    const row = screen.getByText("Nike").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    const input = await screen.findByLabelText("Brand Name");
    expect(input).toHaveValue("Nike");
    fireEvent.change(input, { target: { value: "Nike USA" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      const bodies = getBodies();
      expect(bodies.some((b) => b.query.includes("updateBrand") && b.variables.id === "1")).toBe(true);
    });
  });

  it("deletes a brand", async () => {
    renderPage(<BrandsPage />);
    await waitFor(() => screen.getByText("Nike"));
    const row = screen.getByText("Nike").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteBrand") && b.variables.id === "1")).toBe(true);
    });
  });
});

describe("flavor profiles page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createFlavorProfile")) {
        return Promise.resolve(gql({ createFlavorProfile: { id: "3", name: "Salty", isActive: true } }));
      }
      if (body.query.includes("updateFlavorProfile")) {
        return Promise.resolve(gql({ updateFlavorProfile: { id: "1", name: "Sweet & Sour", isActive: true } }));
      }
      if (body.query.includes("deleteFlavorProfile")) {
        return Promise.resolve(gql({ deleteFlavorProfile: true }));
      }
      return Promise.resolve(
        gql({
          flavorProfiles: [
            { id: "1", name: "Sweet", isActive: true },
            { id: "2", name: "Retired", isActive: false },
          ],
        })
      );
    });
  });

  it("lists flavor profiles", async () => {
    renderPage(<FlavorProfilesPage />);
    await waitFor(() => expect(screen.getByText("Sweet")).toBeInTheDocument());
    expect(screen.getByText("Retired")).toBeInTheDocument();
  });

  it("creates a flavor profile", async () => {
    renderPage(<FlavorProfilesPage />);
    await waitFor(() => screen.getByText("Sweet"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Flavor Name"), { target: { value: "Salty" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createFlavorProfile"))).toBe(true);
    });
  });

  it("edits a flavor profile", async () => {
    renderPage(<FlavorProfilesPage />);
    await waitFor(() => screen.getByText("Sweet"));
    const row = screen.getByText("Sweet").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    const input = await screen.findByLabelText("Flavor Name");
    fireEvent.change(input, { target: { value: "Sweet & Sour" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateFlavorProfile") && b.variables.id === "1")).toBe(true);
    });
  });

  it("deletes a flavor profile", async () => {
    renderPage(<FlavorProfilesPage />);
    await waitFor(() => screen.getByText("Sweet"));
    const row = screen.getByText("Sweet").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteFlavorProfile") && b.variables.id === "1")).toBe(true);
    });
  });

  it("filters to active flavor profiles", async () => {
    renderPage(<FlavorProfilesPage />);
    await waitFor(() => screen.getByText("Retired"));
    fireEvent.click(screen.getByLabelText("Active only"));
    await waitFor(() => expect(screen.queryByText("Retired")).not.toBeInTheDocument());
    expect(screen.getByText("Sweet")).toBeInTheDocument();
  });
});

describe("nutrient types page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createNutrientType")) {
        return Promise.resolve(gql({ createNutrientType: { id: "2", name: "Protein", unit: "g" } }));
      }
      if (body.query.includes("updateNutrientType")) {
        return Promise.resolve(gql({ updateNutrientType: { id: "1", name: "Sugar", unit: "mg" } }));
      }
      if (body.query.includes("deleteNutrientType")) {
        return Promise.resolve(gql({ deleteNutrientType: true }));
      }
      return Promise.resolve(gql({ nutrientTypes: [{ id: "1", name: "Sugar", unit: "g" }] }));
    });
  });

  it("lists nutrient types", async () => {
    renderPage(<NutrientTypesPage />);
    await waitFor(() => expect(screen.getByText("Sugar")).toBeInTheDocument());
    expect(screen.getByText("g")).toBeInTheDocument();
  });

  it("creates a nutrient type", async () => {
    renderPage(<NutrientTypesPage />);
    await waitFor(() => screen.getByText("Sugar"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Nutrient Name"), { target: { value: "Protein" } });
    fireEvent.change(screen.getByLabelText("Unit of Measure"), { target: { value: "g" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createNutrientType"))).toBe(true);
    });
  });

  it("edits a nutrient type", async () => {
    renderPage(<NutrientTypesPage />);
    await waitFor(() => screen.getByText("Sugar"));
    const row = screen.getByText("Sugar").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Unit of Measure"), { target: { value: "mg" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateNutrientType") && b.variables.id === "1")).toBe(true);
    });
  });

  it("deletes a nutrient type", async () => {
    renderPage(<NutrientTypesPage />);
    await waitFor(() => screen.getByText("Sugar"));
    const row = screen.getByText("Sugar").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteNutrientType") && b.variables.id === "1")).toBe(true);
    });
  });
});

describe("food flavors page", () => {
  const item = {
    id: "1",
    name: "Yogurt",
    upc12: null,
    upc14: null,
    unit: "cup",
    brand: { id: "2", name: "DairyCo" },
    category: { id: "3", name: "Dairy", description: null },
    nutrients: [],
    flavors: [{ intensity: 5, flavor: { id: "1", name: "Sweet", isActive: true } }],
  };

  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("removeFoodFlavor")) {
        return Promise.resolve(gql({ removeFoodFlavor: true }));
      }
      if (body.query.includes("addFoodFlavor")) {
        return Promise.resolve(
          gql({ addFoodFlavor: { intensity: 7, flavor: { id: "1", name: "Sweet", isActive: true } } })
        );
      }
      return Promise.resolve(
        gql({
          items: {
            items: [item],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 },
          },
        })
      );
    });
  });

  it("lists food flavors", async () => {
    renderPage(<FoodFlavorsPage />);
    await waitFor(() => expect(screen.getByText("5")).toBeInTheDocument());
    const bodies = getBodies();
    expect(bodies.some((b) => b.query.includes("items"))).toBe(true);
  });

  it("creates a food flavor", async () => {
    renderPage(<FoodFlavorsPage />);
    await waitFor(() => screen.getByText("5"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Food ID"), { target: { value: "1" } });
    fireEvent.change(screen.getByLabelText("Flavor ID"), { target: { value: "1" } });
    fireEvent.change(screen.getByLabelText("Intensity Score"), { target: { value: "7" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("addFoodFlavor"))).toBe(true);
    });
  });

  it("edits a food flavor", async () => {
    renderPage(<FoodFlavorsPage />);
    await waitFor(() => screen.getByText("5"));
    const row = screen.getByText("5").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Intensity Score"), { target: { value: "8" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      const bodies = getBodies();
      expect(bodies.some((b) => b.query.includes("removeFoodFlavor"))).toBe(true);
      expect(bodies.some((b) => b.query.includes("addFoodFlavor"))).toBe(true);
    });
  });

  it("deletes a food flavor", async () => {
    renderPage(<FoodFlavorsPage />);
    await waitFor(() => screen.getByText("5"));
    const row = screen.getByText("5").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("removeFoodFlavor"))).toBe(true);
    });
  });
});

describe("food nutrients page", () => {
  const item = {
    id: "1",
    name: "Yogurt",
    upc12: null,
    upc14: null,
    unit: "cup",
    brand: { id: "2", name: "DairyCo" },
    category: { id: "3", name: "Dairy", description: null },
    nutrients: [{ amount: 10, nutrient: { id: "1", name: "Sugar", unit: "g" } }],
    flavors: [],
  };

  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("removeFoodNutrient")) {
        return Promise.resolve(gql({ removeFoodNutrient: true }));
      }
      if (body.query.includes("addFoodNutrient")) {
        return Promise.resolve(
          gql({ addFoodNutrient: { amount: 12, nutrient: { id: "1", name: "Sugar", unit: "g" } } })
        );
      }
      return Promise.resolve(
        gql({
          items: {
            items: [item],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 1 },
          },
        })
      );
    });
  });

  it("lists food nutrients", async () => {
    renderPage(<FoodNutrientsPage />);
    await waitFor(() => expect(screen.getByText("10")).toBeInTheDocument());
    const bodies = getBodies();
    expect(bodies.some((b) => b.query.includes("items"))).toBe(true);
  });

  it("creates a food nutrient", async () => {
    renderPage(<FoodNutrientsPage />);
    await waitFor(() => screen.getByText("10"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Food ID"), { target: { value: "1" } });
    fireEvent.change(screen.getByLabelText("Nutrient ID"), { target: { value: "1" } });
    fireEvent.change(screen.getByLabelText("Amount per Serving"), { target: { value: "12" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("addFoodNutrient"))).toBe(true);
    });
  });

  it("edits a food nutrient", async () => {
    renderPage(<FoodNutrientsPage />);
    await waitFor(() => screen.getByText("10"));
    const row = screen.getByText("10").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Amount per Serving"), { target: { value: "15" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      const bodies = getBodies();
      expect(bodies.some((b) => b.query.includes("removeFoodNutrient"))).toBe(true);
      expect(bodies.some((b) => b.query.includes("addFoodNutrient"))).toBe(true);
    });
  });

  it("deletes a food nutrient", async () => {
    renderPage(<FoodNutrientsPage />);
    await waitFor(() => screen.getByText("10"));
    const row = screen.getByText("10").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("removeFoodNutrient"))).toBe(true);
    });
  });
});
