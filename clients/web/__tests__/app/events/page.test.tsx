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
    await waitFor(() => screen.getByText("Nothing here yet"));

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
      .mockResolvedValueOnce(gql({ aiAvailable: false }))
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
      .mockResolvedValueOnce(gql({ aiAvailable: false }))
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

  const gqlEventWithSteps = {
    ...gqlEvent,
    recipes: [
      {
        ...gqlEvent.recipes[0],
        steps: [
          {
            id: "50",
            stepNumber: 1,
            instruction: "Roast turkey",
            durationMinutes: 60,
            stepType: "cook",
            isPassive: true,
            dependsOnStepNumber: null,
            appliance: "oven",
          },
        ],
        items: [],
      },
    ],
  };

  const gqlTimeline = {
    eventTimeline: {
      foodEventId: "3",
      warnings: ['appliance conflict: oven is needed by "Turkey" step 1'],
      recipes: [
        {
          eventRecipeId: "9",
          name: "Turkey",
          targetTime: "2026-11-26T18:30:00Z",
          servings: 8,
          baseServings: null,
          startBy: "2026-11-26T17:30:00Z",
          unschedulable: false,
          warnings: [],
          steps: [
            {
              stepNumber: 1,
              instruction: "Roast turkey",
              stepType: "cook",
              isPassive: true,
              appliance: "oven",
              durationMinutes: 60,
              scheduledMinutes: 60,
              estimated: false,
              startTime: "2026-11-26T17:30:00Z",
              endTime: "2026-11-26T18:30:00Z",
              conflicts: ['oven is needed by "Turkey" step 1'],
            },
          ],
        },
      ],
    },
  };

  it("suggests schedule fixes and applies a shift via updateEventRecipe", async () => {
    mockFetch
      .mockResolvedValueOnce(gql({ foodEvent: gqlEventWithSteps }))
      .mockResolvedValueOnce(gql({ aiAvailable: true }))
      .mockResolvedValueOnce(
        gql({
          recipes: {
            items: [],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 },
          },
        })
      )
      .mockResolvedValueOnce(gql(gqlTimeline))
      .mockResolvedValueOnce(
        gql({
          suggestEventFixes: [
            {
              eventRecipeId: "9",
              recipeName: "Turkey",
              stepNumber: null,
              action: "shift_serve",
              minutes: 30,
              appliance: null,
              durationMinutes: null,
              dependsOnStepNumber: null,
              reason: "frees the oven slot",
            },
            {
              eventRecipeId: "9",
              recipeName: "Turkey",
              stepNumber: 1,
              action: "set_appliance",
              minutes: null,
              appliance: "grill",
              durationMinutes: null,
              dependsOnStepNumber: null,
              reason: "grill is free",
            },
          ],
        })
      )
      .mockResolvedValueOnce(
        gql({
          updateEventRecipe: {
            id: "9",
            mealType: "dinner",
            targetTime: "2026-11-26T19:00:00Z",
            servings: 8,
            notes: "turkey",
            recipe: null,
          },
        })
      )
      .mockResolvedValueOnce(gql({ foodEvent: gqlEventWithSteps }))
      .mockResolvedValueOnce(gql(gqlTimeline));

    await renderDetailPage(
      <EventDetailPage params={Promise.resolve({ id: "3" })} />
    );
    await waitFor(() => screen.getByText("Friendsgiving"));

    fireEvent.click(screen.getByRole("button", { name: "Generate Timeline" }));
    await waitFor(() => screen.getByRole("button", { name: "Suggest Fixes" }));
    fireEvent.click(screen.getByRole("button", { name: "Suggest Fixes" }));

    await waitFor(() => screen.getByText("Turkey: serve +30m"));
    expect(screen.getByText("Turkey, step 1: use grill")).toBeInTheDocument();
    expect(screen.getByText("frees the oven slot")).toBeInTheDocument();

    // Apply the shift — goes through updateEventRecipe with the moved
    // targetTime, never a direct AI write.
    const applyButtons = screen.getAllByRole("button", { name: "Apply" });
    fireEvent.click(applyButtons[0]);
    await waitFor(() =>
      expect(
        getBodies().some((b) => b.query.includes("updateEventRecipe"))
      ).toBe(true)
    );
    const upd = getBodies().find((b) => b.query.includes("updateEventRecipe"))!;
    expect(upd.variables.id).toBe("9");
    expect(upd.variables.input.targetTime).toBe("2026-11-26T19:00:00.000Z");
    // Applied card is dismissed; the appliance card remains.
    await waitFor(() =>
      expect(screen.queryByText("Turkey: serve +30m")).not.toBeInTheDocument()
    );
    expect(screen.getByText("Turkey, step 1: use grill")).toBeInTheDocument();
  });

  it("applies a step fix via updateEventRecipeStep", async () => {
    mockFetch
      .mockResolvedValueOnce(gql({ foodEvent: gqlEventWithSteps }))
      .mockResolvedValueOnce(gql({ aiAvailable: true }))
      .mockResolvedValueOnce(
        gql({
          recipes: {
            items: [],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 },
          },
        })
      )
      .mockResolvedValueOnce(gql(gqlTimeline))
      .mockResolvedValueOnce(
        gql({
          suggestEventFixes: [
            {
              eventRecipeId: "9",
              recipeName: "Turkey",
              stepNumber: 1,
              action: "set_appliance",
              minutes: null,
              appliance: "grill",
              durationMinutes: null,
              dependsOnStepNumber: null,
              reason: "grill is free",
            },
          ],
        })
      )
      .mockResolvedValueOnce(
        gql({
          updateEventRecipeStep: {
            id: "50",
            stepNumber: 1,
            instruction: "Roast turkey",
            durationMinutes: 60,
            stepType: "cook",
            isPassive: true,
            dependsOnStepNumber: null,
            appliance: "grill",
          },
        })
      )
      .mockResolvedValueOnce(gql({ foodEvent: gqlEventWithSteps }))
      .mockResolvedValueOnce(gql(gqlTimeline));

    await renderDetailPage(
      <EventDetailPage params={Promise.resolve({ id: "3" })} />
    );
    await waitFor(() => screen.getByText("Friendsgiving"));

    fireEvent.click(screen.getByRole("button", { name: "Generate Timeline" }));
    await waitFor(() => screen.getByRole("button", { name: "Suggest Fixes" }));
    fireEvent.click(screen.getByRole("button", { name: "Suggest Fixes" }));
    await waitFor(() => screen.getByText("Turkey, step 1: use grill"));

    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    await waitFor(() =>
      expect(
        getBodies().some((b) => b.query.includes("updateEventRecipeStep"))
      ).toBe(true)
    );
    const upd = getBodies().find((b) =>
      b.query.includes("updateEventRecipeStep")
    )!;
    expect(upd.variables.id).toBe("50");
    expect(upd.variables.input.appliance).toBe("grill");
    // The rest of the step payload is preserved from the row.
    expect(upd.variables.input.instruction).toBe("Roast turkey");
    expect(upd.variables.input.durationMinutes).toBe(60);
  });

  it("hides Suggest Fixes when AI is unavailable", async () => {
    mockFetch
      .mockResolvedValueOnce(gql({ foodEvent: gqlEvent }))
      .mockResolvedValueOnce(gql({ aiAvailable: false }))
      .mockResolvedValueOnce(
        gql({
          recipes: {
            items: [],
            pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 },
          },
        })
      )
      .mockResolvedValueOnce(gql(gqlTimeline));

    await renderDetailPage(
      <EventDetailPage params={Promise.resolve({ id: "3" })} />
    );
    await waitFor(() => screen.getByText("Friendsgiving"));
    fireEvent.click(screen.getByRole("button", { name: "Generate Timeline" }));
    await waitFor(() => screen.getByText("Turkey"));
    expect(
      screen.queryByRole("button", { name: "Suggest Fixes" })
    ).not.toBeInTheDocument();
  });
});
