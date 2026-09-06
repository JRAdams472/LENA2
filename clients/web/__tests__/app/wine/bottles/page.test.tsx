import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import BottlesPage from "@/app/wine/bottles/page";

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

describe("bottles page", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    jest.spyOn(window, "confirm").mockReturnValue(true);

    mockFetch.mockImplementation((_, init) => {
      const body = JSON.parse((init as RequestInit).body as string);
      if (body.query.includes("createBottle")) {
        return Promise.resolve(gql({ createBottle: { ...bottle, id: "2", vineyard: "New Chateau" } }));
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

  it("lists bottles", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Chateau")).toBeInTheDocument());
    const bodies = getBodies();
    expect(bodies.some((b) => b.query.includes("bottles(page:"))).toBe(true);
  });

  it("creates a bottle", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Chateau"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.change(screen.getByLabelText("Type ID"), { target: { value: "1" } });
    fireEvent.change(screen.getByLabelText("Country ID"), { target: { value: "1" } });
    fireEvent.change(screen.getByLabelText("Region ID"), { target: { value: "1" } });
    fireEvent.change(screen.getByLabelText("Vintage Year"), { target: { value: "2015" } });
    fireEvent.change(screen.getByLabelText("Vineyard"), { target: { value: "New Chateau" } });
    fireEvent.change(screen.getByLabelText("Bottle Size"), { target: { value: "750ml" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("createBottle"))).toBe(true);
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
    });
  });

  it("deletes a bottle", async () => {
    renderPage();
    await waitFor(() => screen.getByText("Chateau"));
    const row = screen.getByText("Chateau").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => {
      expect(getBodies().some((b) => b.query.includes("deleteBottle") && b.variables.id === "1")).toBe(true);
    });
  });
});
