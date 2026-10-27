import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import BottlesPage from "@/app/member/wine/bottles/page";

jest.mock("../../../../app/auth/useMe");
import { useMe } from "@/app/auth/useMe";

const mockFetch = global.fetch as jest.Mock;
const mockedUseMe = useMe as jest.Mock;

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
      <BottlesPage />
    </QueryClientProvider>
  );
}

const country = { id: "1", name: "France", isoCode: "FR", description: null };
const region = { id: "1", name: "Bordeaux", description: null, country };
const type = { id: "1", name: "Red", description: null };
const vintage = { id: "1", year: 2015, description: null, isActive: true };

const bottle = {
  id: "1",
  typeId: "1",
  countryId: "1",
  regionId: "1",
  vintageYear: 2015,
  vineyard: "Chateau",
  abv: 13.5,
  acidity: 6,
  tanninLevel: 4,
  body: 3,
  sweetness: 1,
  oakIntegration: true,
  bottleSize: "750ml",
  grapeVarieties: [],
  flavorProfiles: [],
};

const holding = {
  id: "55",
  bottleNumber: 7,
  quantity: 5,
  purchaseAt: "2025-12-01",
  purchasePrice: 42.5,
  storageTemp: 55,
  location: "Rack B",
  notes: "cellared",
  isFavorite: false,
  bottle,
};

describe("bottles page", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockedUseMe.mockReturnValue({ me: { role: "admin" } });
    jest.spyOn(window, "confirm").mockReturnValue(true);

    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("userBottles")) {
        return Promise.resolve(gql({ userBottles: { items: [holding] } }));
      }
      if (body.query.includes("submitBottle")) {
        return Promise.resolve(
          gql({
            submitBottle: { ...bottle, id: "9", vineyard: "Member Vine", status: "pending", submittedByMe: true },
          })
        );
      }
      if (body.query.includes("createBottle")) {
        return Promise.resolve(gql({ createBottle: { ...bottle, id: "2", vineyard: "New Chateau" } }));
      }
      if (body.query.includes("updateUserBottle")) {
        return Promise.resolve(gql({ updateUserBottle: holding }));
      }
      if (body.query.includes("updateBottle")) {
        return Promise.resolve(gql({ updateBottle: { ...bottle, vineyard: "Chateau Updated" } }));
      }
      if (body.query.includes("deleteBottle")) {
        return Promise.resolve(gql({ deleteBottle: true }));
      }
      if (body.query.includes("bottles(page:")) {
        if (body.variables.page === 1 && body.variables.pageSize === 1) {
          return Promise.resolve(
            gql({ bottles: { pageInfo: { pageNumber: 1, pageSize: 1, totalCount: 1 } } })
          );
        }
        return Promise.resolve(
          gql({
            bottles: {
              items: [bottle],
              pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
            },
          })
        );
      }
      if (body.query.includes("regions(")) {
        return Promise.resolve(gql({ regions: [region] }));
      }
      if (body.query.includes("types")) {
        return Promise.resolve(gql({ types: [type] }));
      }
      if (body.query.includes("vintages")) {
        return Promise.resolve(gql({ vintages: [vintage] }));
      }
      return Promise.resolve(gql({ countries: [country] }));
    });
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("lists bottles with the cellar overlay", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Chateau")).toBeInTheDocument());
    const bodies = getBodies();
    expect(bodies.some((b) => b.query.includes("bottles(page:"))).toBe(true);
    expect(bodies.some((b) => b.query.includes("userBottles"))).toBe(true);
    expect(screen.getByText("Rack B")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("creates a bottle", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Chateau"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.mouseDown(await screen.findByRole("combobox", { name: "Type" }));
    fireEvent.click(await screen.findByRole("option", { name: "Red" }));
    fireEvent.change(screen.getByLabelText("Vineyard"), { target: { value: "New Chateau" } });
    fireEvent.change(screen.getByLabelText("Bottle Size"), { target: { value: "750ml" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      const call = getBodies().find((b) => b.query.includes("createBottle"));
      expect(call?.variables.input.typeId).toBe("1");
    });
  });

  it("edits a bottle", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Chateau"));
    const row = screen.getByText("Chateau").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    fireEvent.change(await screen.findByLabelText("Vineyard"), { target: { value: "Chateau Updated" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("updateBottle") && b.variables.id === "1")).toBe(true);
      expect(getBodies().some((b) => b.query.includes("updateUserBottle") && b.variables.bottleId === "1")).toBe(true);
    });
  });

  it("deletes a bottle after confirming", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Chateau"));
    const row = screen.getByText("Chateau").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    expect(
      getBodies().some((b) => b.query.includes("deleteBottle"))
    ).toBe(false);
    const confirm = await screen.findByRole("dialog", { name: "Delete bottle" });
    fireEvent.click(within(confirm).getByRole("button", { name: "Delete" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteBottle") && b.variables.id === "1")).toBe(true);
    });
  });

  it("scopes the region filter to the selected country", async () => {
    const usa = { id: "2", name: "USA", isoCode: "US", description: null };
    const napa = { id: "2", name: "Napa Valley", description: null, country: usa };
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("userBottles")) {
        return Promise.resolve(gql({ userBottles: { items: [holding] } }));
      }
      if (body.query.includes("bottles(page:")) {
        return Promise.resolve(
          gql({
            bottles: {
              items: [bottle],
              pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
            },
          })
        );
      }
      if (body.query.includes("regions(")) {
        // getRegions fans out one regions(countryId) call per country.
        const byCountry = body.variables.countryId === "2" ? napa : region;
        return Promise.resolve(gql({ regions: [byCountry] }));
      }
      if (body.query.includes("types")) {
        return Promise.resolve(gql({ types: [type] }));
      }
      if (body.query.includes("vintages")) {
        return Promise.resolve(gql({ vintages: [vintage] }));
      }
      return Promise.resolve(gql({ countries: [country, usa] }));
    });

    renderPage();
    await waitFor(() => screen.getByText("Chateau"));

    // Unscoped: every region is offered.
    fireEvent.mouseDown(screen.getByLabelText("Region"));
    const allOptions = await screen.findAllByRole("option");
    expect(allOptions.map((o) => o.textContent)).toEqual([
      "All",
      "Bordeaux",
      "Napa Valley",
    ]);
    // Pick the USA region, then switch the country to France — the stale
    // region selection must clear and the list must rescope.
    fireEvent.click(screen.getByRole("option", { name: "Napa Valley" }));
    fireEvent.mouseDown(screen.getByLabelText("Country"));
    fireEvent.click(await screen.findByRole("option", { name: "France" }));

    fireEvent.mouseDown(screen.getByLabelText("Region"));
    const franceOptions = await screen.findAllByRole("option");
    expect(franceOptions.map((o) => o.textContent)).toEqual(["All", "Bordeaux"]);
    // The reset also drops the region filter from the next fetch.
    const lastListCall = getBodies()
      .filter((b) => b.query.includes("bottles(page:"))
      .pop();
    expect(lastListCall?.variables.regionId ?? null).toBeNull();
  });

  it("member sees cellar fields only and saves via updateUserBottle", async () => {
    mockedUseMe.mockReturnValue({ me: { role: "member" } });
    renderPage();
    await waitFor(() => screen.getByText("Chateau"));
    const row = screen.getByText("Chateau").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[0]);
    expect(await screen.findByLabelText("Quantity")).toBeInTheDocument();
    expect(screen.queryByLabelText("Vineyard")).toBeNull();
    fireEvent.change(screen.getByLabelText("Location"), { target: { value: "Rack C" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      const bodies = getBodies();
      expect(bodies.some((b) => b.query.includes("updateBottle("))).toBe(false);
      const cellar = bodies.find((b) => b.query.includes("updateUserBottle"));
      expect(cellar?.variables.bottleId).toBe("1");
      expect(cellar?.variables.input.location).toBe("Rack C");
    });
  });

  it("member has no delete action", async () => {
    mockedUseMe.mockReturnValue({ me: { role: "member" } });
    renderPage();
    await waitFor(() => screen.getByText("Chateau"));
    const row = screen.getByText("Chateau").closest("tr")!;
    // Member rows expose only the edit action.
    expect(row.querySelectorAll("button")).toHaveLength(1);
  });

  it("member can submit a new bottle for review and add it to the cellar", async () => {
    mockedUseMe.mockReturnValue({ me: { role: "member" } });
    // The cellar picker's catalog search finds nothing, surfacing the
    // "Submit new bottle" action in the empty state.
    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("submitBottle")) {
        return Promise.resolve(
          gql({
            submitBottle: { ...bottle, id: "9", vineyard: "Member Vine", status: "pending", submittedByMe: true },
          })
        );
      }
      if (body.query.includes("updateUserBottle")) {
        return Promise.resolve(gql({ updateUserBottle: { ...holding, bottle: { ...bottle, id: "9" } } }));
      }
      if (body.query.includes("userBottles")) {
        return Promise.resolve(
          gql({
            userBottles: {
              items: [holding],
              pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
            },
          })
        );
      }
      if (body.query.includes("bottles(page:")) {
        if (body.variables.page === 1 && body.variables.pageSize === 1) {
          return Promise.resolve(
            gql({ bottles: { pageInfo: { pageNumber: 1, pageSize: 1, totalCount: 1 } } })
          );
        }
        if (body.variables.pageSize === 50) {
          // Cellar picker's searchBottles call — empty so the "Submit new
          // bottle" empty-state action surfaces.
          return Promise.resolve(
            gql({
              bottles: {
                items: [],
                pageInfo: { pageNumber: 1, pageSize: 50, totalCount: 0 },
              },
            })
          );
        }
        return Promise.resolve(
          gql({
            bottles: {
              items: [bottle],
              pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
            },
          })
        );
      }
      if (body.query.includes("regions(")) {
        return Promise.resolve(gql({ regions: [region] }));
      }
      if (body.query.includes("types")) {
        return Promise.resolve(gql({ types: [type] }));
      }
      if (body.query.includes("vintages")) {
        return Promise.resolve(gql({ vintages: [vintage] }));
      }
      return Promise.resolve(gql({ countries: [country] }));
    });

    renderPage();
    await waitFor(() => screen.getByText("Rack B"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));

    // Open the cellar autocomplete; empty results offer the submit action.
    fireEvent.mouseDown(await screen.findByLabelText("Bottle"));
    fireEvent.click(await screen.findByText("Submit new bottle"));

    const dlg = await screen.findByRole("dialog", { name: /submit new bottle/i });
    fireEvent.mouseDown(within(dlg).getByRole("combobox", { name: "Type" }));
    fireEvent.click(await screen.findByRole("option", { name: "Red" }));
    fireEvent.mouseDown(within(dlg).getByRole("combobox", { name: "Country" }));
    fireEvent.click(await screen.findByRole("option", { name: "France" }));
    fireEvent.mouseDown(within(dlg).getByRole("combobox", { name: "Region" }));
    fireEvent.click(await screen.findByRole("option", { name: "Bordeaux" }));
    fireEvent.mouseDown(within(dlg).getByRole("combobox", { name: "Vintage" }));
    fireEvent.click(await screen.findByRole("option", { name: "2015" }));
    fireEvent.change(within(dlg).getByLabelText("Vineyard"), { target: { value: "Member Vine" } });
    fireEvent.click(within(dlg).getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const call = getBodies().find((b) => b.query.includes("submitBottle"));
      expect(call?.variables.input).toMatchObject({
        typeId: "1",
        countryId: "1",
        regionId: "1",
        vintageYear: 2015,
        vineyard: "Member Vine",
      });
    });

    // The pending bottle is selected in the cellar dialog with a notice.
    // Wait for the submit dialog's exit transition to unmount it — while it
    // is still mounted, the cellar dialog behind it is aria-hidden and its
    // Save button is unreachable to role queries.
    await screen.findByText(/submitted for review/i);
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: /submit new bottle/i })
      ).not.toBeInTheDocument()
    );
    const cellarDlg = screen.getByRole("dialog", { name: /add bottle to cellar/i });
    fireEvent.click(within(cellarDlg).getByRole("button", { name: "Save" }));
    await waitFor(() => {
      const cellar = getBodies().find(
        (b) => b.query.includes("updateUserBottle") && b.variables.bottleId === "9"
      );
      expect(cellar).toBeTruthy();
    });
  });
});
