import "@testing-library/jest-dom";
import { Suspense } from "react";
import userEvent from "@testing-library/user-event";
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
    expect(screen.getByText("30 min")).toBeInTheDocument();

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

  describe("expanded slot management", () => {
    const gqlEventFull = {
      ...gqlEvent,
      recipes: [
        {
          ...gqlEventWithSteps.recipes[0],
          recipe: {
            id: "5",
            name: "Turkey",
            description: null,
            servings: 12,
            prepTimeMinutes: null,
            cookTimeMinutes: 240,
            isFavorite: false,
            items: [],
            steps: [],
            categories: [],
            allergens: [],
            allergyWarnings: [],
          },
          baseServings: 12,
          scalingFactor: 0.67,
          items: [
            {
              id: "60",
              quantity: 2,
              baseQuantity: 2,
              unit: "lb",
              section: "main",
              displayOrder: 0,
              notes: null,
              isOptional: false,
              item: { id: "1", name: "Turkey breast" },
              ingredient: null,
            },
          ],
        },
      ],
    };

    const searchItem = {
      id: "1",
      name: "Turkey breast",
      upc12: null,
      upc14: null,
      unit: "lb",
      brand: null,
      category: null,
      nutrients: [],
      flavors: [],
    };

    const newStep = {
      id: "51",
      stepNumber: 2,
      instruction: "Rest",
      durationMinutes: 20,
      stepType: null,
      isPassive: true,
      dependsOnStepNumber: null,
      appliance: null,
    };

    function mockDetail() {
      mockFetch.mockImplementation((_, init) => {
        const body = JSON.parse((init as RequestInit).body as string);
        // Step/item mutation names contain the add/remove/updateEventRecipe
        // prefixes — check the longer names first.
        if (body.query.includes("addEventRecipeStep")) {
          return Promise.resolve(gql({ addEventRecipeStep: newStep }));
        }
        if (body.query.includes("updateEventRecipeStep")) {
          return Promise.resolve(
            gql({ updateEventRecipeStep: gqlEventWithSteps.recipes[0].steps[0] })
          );
        }
        if (body.query.includes("removeEventRecipeStep")) {
          return Promise.resolve(gql({ removeEventRecipeStep: true }));
        }
        if (body.query.includes("addEventRecipeItem")) {
          return Promise.resolve(
            gql({
              addEventRecipeItem: {
                id: "61",
                quantity: 3,
                baseQuantity: 3,
                unit: "lb",
                section: null,
                displayOrder: 1,
                notes: null,
                isOptional: false,
                item: { id: "1", name: "Turkey breast" },
                ingredient: null,
              },
            })
          );
        }
        if (body.query.includes("updateEventRecipeItem")) {
          return Promise.resolve(
            gql({ updateEventRecipeItem: gqlEventFull.recipes[0].items[0] })
          );
        }
        if (body.query.includes("removeEventRecipeItem")) {
          return Promise.resolve(gql({ removeEventRecipeItem: true }));
        }
        if (body.query.includes("syncEventRecipe")) {
          return Promise.resolve(
            gql({ syncEventRecipe: gqlEventFull.recipes[0] })
          );
        }
        if (body.query.includes("updateEventRecipe")) {
          return Promise.resolve(
            gql({ updateEventRecipe: gqlEventFull.recipes[0] })
          );
        }
        if (body.query.includes("removeEventRecipe")) {
          return Promise.resolve(gql({ removeEventRecipe: true }));
        }
        if (body.query.includes("items(")) {
          return Promise.resolve(gql({ items: { items: [searchItem] } }));
        }
        if (body.query.includes("recordSelection")) {
          return Promise.resolve(gql({ recordSelection: true }));
        }
        if (body.query.includes("aiAvailable")) {
          return Promise.resolve(gql({ aiAvailable: false }));
        }
        if (body.query.includes("eventTimeline")) {
          return Promise.resolve(gql(gqlTimeline));
        }
        // `recipes(` with paren — the foodEvent query selects `recipes {`
        // on the event itself and must not be intercepted.
        if (body.query.includes("recipes(")) {
          return Promise.resolve(
            gql({ recipes: { items: [], pageInfo: { pageNumber: 1, pageSize: 200, totalCount: 0 } } })
          );
        }
        return Promise.resolve(gql({ foodEvent: gqlEventFull }));
      });
    }

    async function renderExpanded() {
      await renderDetailPage(
        <EventDetailPage params={Promise.resolve({ id: "3" })} />
      );
      await waitFor(() => screen.getByText("Friendsgiving"));
      // The steps-count button toggles the expanded row.
      fireEvent.click(screen.getByRole("button", { name: /^1 ▼$/ }));
      await screen.findByText(/Event-specific steps/);
    }

    it("expands a slot to show steps, scaled items, and sync", async () => {
      mockDetail();
      window.confirm = jest.fn(() => true);
      await renderExpanded();

      expect(screen.getByText("Roast turkey")).toBeInTheDocument();
      expect(screen.getByText("Turkey breast")).toBeInTheDocument();
      expect(
        screen.getByText(/scaled for 8 servings \(recipe makes 12\)/)
      ).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Sync from recipe" }));
      await waitFor(() =>
        expect(getBodies().some((b) => b.query.includes("syncEventRecipe"))).toBe(true)
      );
    });

    it("adds a step through the dialog", async () => {
      mockDetail();
      await renderExpanded();

      fireEvent.click(screen.getByRole("button", { name: "Add step" }));
      await screen.findByText("Add Step");
      fireEvent.change(screen.getByLabelText("Instruction"), {
        target: { value: "Rest" },
      });
      fireEvent.change(screen.getByLabelText("Duration (minutes)"), {
        target: { value: "20" },
      });
      fireEvent.click(screen.getByRole("checkbox", { name: /Hands-off/ }));
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
      await waitFor(() => {
        const body = getBodies().find((b) => b.query.includes("addEventRecipeStep"));
        expect(body?.variables.input.instruction).toBe("Rest");
        expect(body?.variables.input.durationMinutes).toBe(20);
        expect(body?.variables.input.isPassive).toBe(true);
      });
    });

    it("edits and removes a step", async () => {
      mockDetail();
      await renderExpanded();

      fireEvent.click(screen.getByLabelText("Edit step"));
      await screen.findByText("Edit Step");
      fireEvent.change(screen.getByLabelText("Instruction"), {
        target: { value: "Roast low and slow" },
      });
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
      await waitFor(() => {
        const body = getBodies().find((b) => b.query.includes("updateEventRecipeStep"));
        expect(body?.variables.id).toBe("50");
        expect(body?.variables.input.instruction).toBe("Roast low and slow");
      });

      fireEvent.click(screen.getByLabelText("Delete step"));
      await waitFor(() =>
        expect(
          getBodies().some((b) => b.query.includes("removeEventRecipeStep"))
        ).toBe(true)
      );
    });

    it("adds an ingredient through the dialog", async () => {
      mockDetail();
      await renderExpanded();

      fireEvent.click(screen.getByRole("button", { name: "Add ingredient" }));
      await screen.findByText("Add Ingredient");

      // MUI Autocomplete needs real keystrokes to reach onInputChange.
      const itemInput = screen.getByLabelText("Item");
      await userEvent.type(itemInput, "tur");
      await waitFor(
        () => expect(getBodies().some((b) => b.query.includes("items("))).toBe(true),
        { timeout: 3000 }
      );
      await userEvent.click(
        await screen.findByRole("option", { name: /Turkey breast/ })
      );

      fireEvent.change(screen.getByLabelText("Quantity (per recipe serving)"), {
        target: { value: "3" },
      });
      fireEvent.change(screen.getByLabelText("Unit"), { target: { value: "lb" } });
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
      await waitFor(() => {
        const body = getBodies().find((b) => b.query.includes("addEventRecipeItem"));
        expect(body?.variables.input.itemId).toBe("1");
        expect(body?.variables.input.quantity).toBe(3);
      });
    });

    it("edits and removes an ingredient", async () => {
      mockDetail();
      await renderExpanded();

      fireEvent.click(screen.getByLabelText("Edit ingredient"));
      await screen.findByText("Edit Ingredient");
      fireEvent.change(screen.getByLabelText("Quantity (per recipe serving)"), {
        target: { value: "4" },
      });
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
      await waitFor(() =>
        expect(
          getBodies().some((b) => b.query.includes("updateEventRecipeItem"))
        ).toBe(true)
      );

      fireEvent.click(screen.getByLabelText("Delete ingredient"));
      await waitFor(() =>
        expect(
          getBodies().some((b) => b.query.includes("removeEventRecipeItem"))
        ).toBe(true)
      );
    });

    it("edits a dish slot", async () => {
      mockDetail();
      await renderDetailPage(
        <EventDetailPage params={Promise.resolve({ id: "3" })} />
      );
      await waitFor(() => screen.getByText("Friendsgiving"));

      fireEvent.click(screen.getByLabelText("Edit"));
      await screen.findByText("Edit Dish");
      fireEvent.change(screen.getByLabelText("Servings"), {
        target: { value: "10" },
      });
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
      await waitFor(() => {
        const body = getBodies().find((b) => b.query.includes("updateEventRecipe"));
        expect(body?.variables.id).toBe("9");
        expect(body?.variables.input.servings).toBe(10);
      });
    });

    it("removes a dish slot", async () => {
      mockDetail();
      await renderDetailPage(
        <EventDetailPage params={Promise.resolve({ id: "3" })} />
      );
      await waitFor(() => screen.getByText("Friendsgiving"));

      fireEvent.click(screen.getByLabelText("Delete"));
      await waitFor(() =>
        expect(
          getBodies().some((b) => b.query.includes("removeEventRecipe"))
        ).toBe(true)
      );
    });
  });
});
