import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import CountriesPage from "@/app/wine/countries/page";
import GrapeVarietiesPage from "@/app/wine/grape-varieties/page";
import RegionsPage from "@/app/wine/regions/page";
import TypesPage from "@/app/wine/types/page";
import VintagesPage from "@/app/wine/vintages/page";
import WineFlavorProfilesPage from "@/app/wine/wine-flavor-profiles/page";

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

describe("countries page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createCountry")) {
        return Promise.resolve(gql({ createCountry: { id: "2", name: "Italy", isoCode: "IT", description: null } }));
      }
      if (body.query.includes("updateCountry")) {
        return Promise.resolve(gql({ updateCountry: { id: "1", name: "France", isoCode: "FR", description: "Updated" } }));
      }
      if (body.query.includes("deleteCountry")) {
        return Promise.resolve(gql({ deleteCountry: true }));
      }
      return Promise.resolve(gql({ countries: [{ id: "1", name: "France", isoCode: "FR", description: null }] }));
    });
  });

  it("lists countries", async () => {
    renderPage(<CountriesPage />);
    await waitFor(() => expect(screen.getByText("France")).toBeInTheDocument());
    expect(screen.getByText("FR")).toBeInTheDocument();
  });

  it("creates a country", async () => {
    renderPage(<CountriesPage />);
    await waitFor(() => screen.getByText("France"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Country Name"), { target: { value: "Italy" } });
    fireEvent.change(screen.getByLabelText("Country Code"), { target: { value: "IT" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createCountry"))).toBe(true);
    });
  });

  it("edits a country", async () => {
    renderPage(<CountriesPage />);
    await waitFor(() => screen.getByText("France"));
    const row = screen.getByText("France").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Country Code"), { target: { value: "FRA" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateCountry"))).toBe(true);
    });
  });

  it("deletes a country", async () => {
    renderPage(<CountriesPage />);
    await waitFor(() => screen.getByText("France"));
    const row = screen.getByText("France").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteCountry"))).toBe(true);
    });
  });
});

describe("grape varieties page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createGrapeVariety")) {
        return Promise.resolve(gql({ createGrapeVariety: { id: "3", name: "Syrah", description: null, isActive: true } }));
      }
      if (body.query.includes("updateGrapeVariety")) {
        return Promise.resolve(gql({ updateGrapeVariety: { id: "1", name: "Cabernet Franc", description: "updated", isActive: true } }));
      }
      if (body.query.includes("deleteGrapeVariety")) {
        return Promise.resolve(gql({ deleteGrapeVariety: true }));
      }
      return Promise.resolve(
        gql({
          grapeVarieties: [
            { id: "1", name: "Cabernet Sauvignon", description: null, isActive: true },
            { id: "2", name: "Old Vine", description: null, isActive: false },
          ],
        })
      );
    });
  });

  it("lists grape varieties", async () => {
    renderPage(<GrapeVarietiesPage />);
    await waitFor(() => expect(screen.getByText("Cabernet Sauvignon")).toBeInTheDocument());
  });

  it("creates a grape variety", async () => {
    renderPage(<GrapeVarietiesPage />);
    await waitFor(() => screen.getByText("Cabernet Sauvignon"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Syrah" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createGrapeVariety"))).toBe(true);
    });
  });

  it("edits a grape variety", async () => {
    renderPage(<GrapeVarietiesPage />);
    await waitFor(() => screen.getByText("Cabernet Sauvignon"));
    const row = screen.getByText("Cabernet Sauvignon").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Name"), { target: { value: "Cabernet Franc" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateGrapeVariety"))).toBe(true);
    });
  });

  it("deletes a grape variety", async () => {
    renderPage(<GrapeVarietiesPage />);
    await waitFor(() => screen.getByText("Cabernet Sauvignon"));
    const row = screen.getByText("Cabernet Sauvignon").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteGrapeVariety"))).toBe(true);
    });
  });

  it("filters to active grape varieties", async () => {
    renderPage(<GrapeVarietiesPage />);
    await waitFor(() => screen.getByText("Old Vine"));
    fireEvent.click(screen.getByLabelText("Active only"));
    await waitFor(() => expect(screen.queryByText("Old Vine")).not.toBeInTheDocument());
    expect(screen.getByText("Cabernet Sauvignon")).toBeInTheDocument();
  });
});

describe("regions page", () => {
  const countries = [
    { id: "1", name: "France", isoCode: "FR", description: null },
    { id: "2", name: "Italy", isoCode: "IT", description: null },
  ];

  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createRegion")) {
        return Promise.resolve(
          gql({ createRegion: { id: "3", name: "Tuscany", description: null, country: { id: "2", name: "Italy", isoCode: "IT", description: null } } })
        );
      }
      if (body.query.includes("updateRegion")) {
        return Promise.resolve(
          gql({ updateRegion: { id: "1", name: "Bordeaux", description: "updated", country: { id: "1", name: "France", isoCode: "FR", description: null } } })
        );
      }
      if (body.query.includes("deleteRegion")) {
        return Promise.resolve(gql({ deleteRegion: true }));
      }
      if (body.query.includes("regions(")) {
        const countryId = body.variables.countryId;
        if (countryId === "1") {
          return Promise.resolve(
            gql({ regions: [{ id: "1", name: "Bordeaux", description: null, country: countries[0] }] })
          );
        }
        if (countryId === "2") {
          return Promise.resolve(
            gql({ regions: [{ id: "2", name: "Tuscany", description: null, country: countries[1] }] })
          );
        }
        return Promise.resolve(gql({ regions: [] }));
      }
      return Promise.resolve(gql({ countries }));
    });
  });

  it("lists regions", async () => {
    renderPage(<RegionsPage />);
    await waitFor(() => expect(screen.getByText("Bordeaux")).toBeInTheDocument());
  });

  it("creates a region", async () => {
    renderPage(<RegionsPage />);
    await waitFor(() => screen.getByText("Bordeaux"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Region Name"), { target: { value: "Tuscany" } });
    fireEvent.change(screen.getByLabelText("Description"), { target: { value: "Italy" } });
    fireEvent.change(screen.getByLabelText("Country ID"), { target: { value: "2" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createRegion"))).toBe(true);
    });
  });

  it("edits a region", async () => {
    renderPage(<RegionsPage />);
    await waitFor(() => screen.getByText("Bordeaux"));
    const row = screen.getByText("Bordeaux").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Region Name"), { target: { value: "Bordeaux AOC" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateRegion"))).toBe(true);
    });
  });

  it("deletes a region", async () => {
    renderPage(<RegionsPage />);
    await waitFor(() => screen.getByText("Bordeaux"));
    const row = screen.getByText("Bordeaux").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteRegion"))).toBe(true);
    });
  });
});

describe("types page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createType")) {
        return Promise.resolve(gql({ createType: { id: "2", name: "Sparkling", description: null } }));
      }
      if (body.query.includes("updateType")) {
        return Promise.resolve(gql({ updateType: { id: "1", name: "Red Wine", description: "updated" } }));
      }
      if (body.query.includes("deleteType")) {
        return Promise.resolve(gql({ deleteType: true }));
      }
      return Promise.resolve(gql({ types: [{ id: "1", name: "Red", description: null }] }));
    });
  });

  it("lists types", async () => {
    renderPage(<TypesPage />);
    await waitFor(() => expect(screen.getByText("Red")).toBeInTheDocument());
  });

  it("creates a type", async () => {
    renderPage(<TypesPage />);
    await waitFor(() => screen.getByText("Red"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Type Name"), { target: { value: "Sparkling" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createType"))).toBe(true);
    });
  });

  it("edits a type", async () => {
    renderPage(<TypesPage />);
    await waitFor(() => screen.getByText("Red"));
    const row = screen.getByText("Red").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Type Name"), { target: { value: "Red Wine" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateType"))).toBe(true);
    });
  });

  it("deletes a type", async () => {
    renderPage(<TypesPage />);
    await waitFor(() => screen.getByText("Red"));
    const row = screen.getByText("Red").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteType"))).toBe(true);
    });
  });
});

describe("vintages page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createVintage")) {
        return Promise.resolve(gql({ createVintage: { id: "3", year: 2022, description: null, isActive: true } }));
      }
      if (body.query.includes("updateVintage")) {
        return Promise.resolve(gql({ updateVintage: { id: "1", year: 2019, description: "updated", isActive: true } }));
      }
      if (body.query.includes("deleteVintage")) {
        return Promise.resolve(gql({ deleteVintage: true }));
      }
      return Promise.resolve(
        gql({
          vintages: [
            { id: "1", year: 2019, description: null, isActive: true },
            { id: "2", year: 2000, description: null, isActive: false },
          ],
        })
      );
    });
  });

  it("lists vintages", async () => {
    renderPage(<VintagesPage />);
    await waitFor(() => expect(screen.getByText("2019")).toBeInTheDocument());
    expect(screen.getByText("2000")).toBeInTheDocument();
  });

  it("creates a vintage", async () => {
    renderPage(<VintagesPage />);
    await waitFor(() => screen.getByText("2019"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Year"), { target: { value: "2022" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createVintage"))).toBe(true);
    });
  });

  it("edits a vintage", async () => {
    renderPage(<VintagesPage />);
    await waitFor(() => screen.getByText("2019"));
    const row = screen.getByText("2019").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Year"), { target: { value: "2020" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateVintage"))).toBe(true);
    });
  });

  it("deletes a vintage", async () => {
    renderPage(<VintagesPage />);
    await waitFor(() => screen.getByText("2019"));
    const row = screen.getByText("2019").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteVintage"))).toBe(true);
    });
  });

  it("filters to active vintages", async () => {
    renderPage(<VintagesPage />);
    await waitFor(() => screen.getByText("2000"));
    fireEvent.click(screen.getByLabelText("Active only"));
    await waitFor(() => expect(screen.queryByText("2000")).not.toBeInTheDocument());
    expect(screen.getByText("2019")).toBeInTheDocument();
  });
});

describe("wine flavor profiles page", () => {
  beforeEach(() => {
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createWineFlavorProfile")) {
        return Promise.resolve(gql({ createWineFlavorProfile: { id: "3", name: "Oaky", description: null, isActive: true } }));
      }
      if (body.query.includes("updateWineFlavorProfile")) {
        return Promise.resolve(gql({ updateWineFlavorProfile: { id: "1", name: "Fruity", description: "updated", isActive: true } }));
      }
      if (body.query.includes("deleteWineFlavorProfile")) {
        return Promise.resolve(gql({ deleteWineFlavorProfile: true }));
      }
      return Promise.resolve(
        gql({
          wineFlavorProfiles: [
            { id: "1", name: "Fruity", description: null, isActive: true },
            { id: "2", name: "Stale", description: null, isActive: false },
          ],
        })
      );
    });
  });

  it("lists wine flavor profiles", async () => {
    renderPage(<WineFlavorProfilesPage />);
    await waitFor(() => expect(screen.getByText("Fruity")).toBeInTheDocument());
    expect(screen.getByText("Stale")).toBeInTheDocument();
  });

  it("creates a wine flavor profile", async () => {
    renderPage(<WineFlavorProfilesPage />);
    await waitFor(() => screen.getByText("Fruity"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Oaky" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createWineFlavorProfile"))).toBe(true);
    });
  });

  it("edits a wine flavor profile", async () => {
    renderPage(<WineFlavorProfilesPage />);
    await waitFor(() => screen.getByText("Fruity"));
    const row = screen.getByText("Fruity").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Name"), { target: { value: "Fruity Updated" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateWineFlavorProfile"))).toBe(true);
    });
  });

  it("deletes a wine flavor profile", async () => {
    renderPage(<WineFlavorProfilesPage />);
    await waitFor(() => screen.getByText("Fruity"));
    const row = screen.getByText("Fruity").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteWineFlavorProfile"))).toBe(true);
    });
  });

  it("filters to active wine flavor profiles", async () => {
    renderPage(<WineFlavorProfilesPage />);
    await waitFor(() => screen.getByText("Stale"));
    fireEvent.click(screen.getByLabelText("Active only"));
    await waitFor(() => expect(screen.queryByText("Stale")).not.toBeInTheDocument());
    expect(screen.getByText("Fruity")).toBeInTheDocument();
  });
});
