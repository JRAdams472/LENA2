import "@testing-library/jest-dom";
import { Suspense } from "react";
import { act, render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import EventsPage from "@/app/events/page";
import EventDetailPage from "@/app/events/[id]/page";

const mockFetch = global.fetch as jest.Mock;
const mockPush = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: jest.fn(() => ({ push: mockPush })),
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

function renderPage(element: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>{element}</QueryClientProvider>
  );
}

async function renderDetailPage(element: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  let result: ReturnType<typeof render> | undefined;
  await act(async () => {
    result = render(
      <QueryClientProvider client={queryClient}>
        <Suspense fallback={null}>{element}</Suspense>
      </QueryClientProvider>
    );
  });
  return result!;
}

const gqlEvent = {
  id: "3",
  name: "Friendsgiving",
  eventDate: "2026-11-26",
  slotGranularityMinutes: 30,
  isActive: true,
  recipes: [
    {
      id: "9",
      mealType: "dinner",
      targetTime: "2026-11-26T18:30:00Z",
      servings: 8,
      notes: "turkey",
      recipe: null,
    },
  ],
};

describe("EventsPage", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("lists events and creates one through the dialog", async () => {
    mockFetch
      .mockResolvedValueOnce(
        gql({
          foodEvents: {
            items: [gqlEvent],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      )
      .mockResolvedValueOnce(gql({ createFoodEvent: gqlEvent }))
      .mockResolvedValueOnce(
        gql({
          foodEvents: {
            items: [gqlEvent],
            pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 1 },
          },
        })
      );

    renderPage(<EventsPage />);
    await waitFor(() => screen.getByText("Friendsgiving"));
    expect(screen.getByText("30")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Create" }));
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Holiday Dinner" },
    });
    fireEvent.change(screen.getByLabelText("Event Date"), {
      target: { value: "2026-12-24" },
    });
    fireEvent.mouseDown(screen.getByLabelText("Time Slot Granularity"));
    fireEvent.click(await screen.findByRole("option", { name: "30 minutes" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(getBodies().some((b) => b.query.includes("createFoodEvent"))).toBe(true)
    );
    const createBody = getBodies().find((b) => b.query.includes("createFoodEvent"))!;
    expect(createBody.variables.input).toEqual({
      name: "Holiday Dinner",
      eventDate: "2026-12-24",
      slotGranularityMinutes: 30,
    });
  });

  it("keeps Save disabled until name and date are filled", async () => {
    mockFetch.mockResolvedValueOnce(
      gql({
        foodEvents: {
          items: [],
          pageInfo: { pageNumber: 1, pageSize: 25, totalCount: 0 },
        },
      })
    );

    renderPage(<EventsPage />);
    await waitFor(() => screen.getByText("No data"));

    fireEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "X" } });
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Event Date"), {
      target: { value: "2026-12-24" },
    });
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });
});

describe("EventDetailPage", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("renders the event with its dish rows and adds a slot", async () => {
    mockFetch
      .mockResolvedValueOnce(gql({ foodEvent: gqlEvent }))
      .mockResolvedValueOnce(
        gql({
          recipes: {
            items: [],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 },
          },
        })
      )
      .mockResolvedValueOnce(
        gql({
          addEventRecipe: {
            id: "10",
            mealType: "lunch",
            targetTime: "2026-11-26T12:00:00Z",
            servings: null,
            notes: "apps",
            recipe: null,
          },
        })
      )
      .mockResolvedValueOnce(gql({ foodEvent: gqlEvent }));

    await renderDetailPage(
      <EventDetailPage params={Promise.resolve({ id: "3" })} />
    );
    await waitFor(() => screen.getByText("Friendsgiving"));
    expect(screen.getByText("18:30")).toBeInTheDocument();
    expect(screen.getByText("turkey")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Add Dish" }));
    fireEvent.mouseDown(screen.getByLabelText("Meal"));
    fireEvent.click(await screen.findByRole("option", { name: "lunch" }));
    fireEvent.change(screen.getByLabelText("Notes"), {
      target: { value: "apps" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(getBodies().some((b) => b.query.includes("addEventRecipe"))).toBe(true)
    );
    const addBody = getBodies().find((b) => b.query.includes("addEventRecipe"))!;
    expect(addBody.variables.input.mealType).toBe("lunch");
    // 30-minute granularity — default first option 00:00.
    expect(addBody.variables.input.targetTime).toBe("2026-11-26T00:00:00Z");
    expect(addBody.variables.input.notes).toBe("apps");
  });

  it("deletes the event and navigates back to the list", async () => {
    mockFetch
      .mockResolvedValueOnce(gql({ foodEvent: gqlEvent }))
      .mockResolvedValueOnce(
        gql({
          recipes: {
            items: [],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 },
          },
        })
      )
      .mockResolvedValueOnce(gql({ deleteFoodEvent: true }));

    window.confirm = jest.fn(() => true);
    await renderDetailPage(
      <EventDetailPage params={Promise.resolve({ id: "3" })} />
    );
    await waitFor(() => screen.getByText("Friendsgiving"));

    fireEvent.click(screen.getByRole("button", { name: "Delete Event" }));
    await waitFor(() =>
      expect(getBodies().some((b) => b.query.includes("deleteFoodEvent"))).toBe(true)
    );
    await waitFor(() => expect(mockPush).toHaveBeenCalledWith("/events"));
  });
});
