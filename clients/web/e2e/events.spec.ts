import { test, expect, type APIRequestContext } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

/** A date a week out as YYYY-MM-DD, so the event is upcoming. */
function nextWeek(): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + 7);
  return d.toISOString().slice(0, 10);
}

/** Seeds a recipe whose steps carry duration + appliance timing metadata. */
async function seedTimedRecipe(
  request: APIRequestContext,
  token: string,
  name: string,
  steps: {
    instruction: string;
    durationMinutes: number;
    appliance?: string;
    dependsOnStepNumber?: number;
  }[]
): Promise<string> {
  const res = await graphql<{ createRecipe: { id: string } }>(
    request,
    token,
    `mutation ($input: CreateRecipeInput!) {
      createRecipe(input: $input) { id }
    }`,
    {
      input: {
        name,
        servings: 4,
        items: [],
        steps: steps.map((s, i) => ({ stepNumber: i + 1, ...s })),
      },
    }
  );
  return res.createRecipe.id;
}

test.describe("food events", () => {
  test("create an event, add dishes, and read the cooking timeline", async ({
    page,
    request,
  }) => {
    test.setTimeout(90_000);
    const token = await mintToken(request);
    const eventName = unique("E2E Event");
    const roastName = unique("E2E Roast");
    const breadName = unique("E2E Bread");
    const eventDate = nextWeek();

    const roastId = await seedTimedRecipe(request, token, roastName, [
      { instruction: "Season the roast", durationMinutes: 10 },
      {
        instruction: "Roast in the oven",
        durationMinutes: 60,
        appliance: "oven",
        dependsOnStepNumber: 1,
      },
      {
        instruction: "Rest and carve",
        durationMinutes: 15,
        dependsOnStepNumber: 2,
      },
    ]);
    const breadId = await seedTimedRecipe(request, token, breadName, [
      { instruction: "Bake the bread", durationMinutes: 45, appliance: "oven" },
    ]);
    let eventId: string | null = null;

    try {
      // Create the event through the UI.
      await page.goto("/events");
      await page.getByRole("button", { name: "Create", exact: true }).click();
      const createDialog = page.getByRole("dialog");
      await createDialog.getByLabel("Name").fill(eventName);
      await createDialog.getByLabel("Event Date").fill(eventDate);
      await createDialog.getByRole("button", { name: "Save" }).click();

      const row = page.getByRole("row", { name: new RegExp(eventName) });
      await expect(row).toBeVisible();
      await row.getByRole("link", { name: "Manage" }).click();
      await expect(page).toHaveURL(/\/events\/\d+/);
      eventId = page.url().match(/\/events\/(\d+)/)?.[1] ?? null;
      expect(eventId).not.toBeNull();

      // Add the roast slot through the dish dialog.
      await page.getByRole("button", { name: "Add Dish" }).click();
      const dish = page.getByRole("dialog");
      await dish.getByLabel("Recipe").fill(roastName);
      await page.getByRole("option", { name: roastName }).click();
      await dish.getByLabel("Serve At").click();
      await page.getByRole("option", { name: "18:00", exact: true }).click();
      await dish.getByLabel("Servings").fill("6");
      await dish.getByRole("button", { name: "Save" }).click();
      await expect(dish).toBeHidden();
      await expect(
        page.getByRole("row", { name: new RegExp(roastName) })
      ).toContainText("18:00");

      // The second oven dish lands via GraphQL at the same serve time —
      // both demand the oven over overlapping windows.
      await graphql(
        request,
        token,
        `mutation ($input: AddEventRecipeInput!) {
          addEventRecipe(input: $input) { id }
        }`,
        {
          input: {
            foodEventId: eventId,
            recipeId: breadId,
            mealType: "dinner",
            targetTime: `${eventDate}T18:00:00Z`,
            servings: 4,
          },
        }
      );
      await page.reload();

      await page.getByRole("button", { name: "Generate Timeline" }).click();

      // Steps render in schedule order inside each dish card.
      const roastCard = page
        .locator("div")
        .filter({ has: page.getByRole("heading", { name: roastName }) })
        .filter({ hasText: "Rest and carve" })
        .last();
      const order = await roastCard
        .getByText(/Season the roast|Roast in the oven|Rest and carve/)
        .allTextContents();
      const idx = (s: string) => order.findIndex((t) => t.includes(s));
      expect(idx("Season the roast")).toBeGreaterThanOrEqual(0);
      expect(idx("Season the roast")).toBeLessThan(idx("Roast in the oven"));
      expect(idx("Roast in the oven")).toBeLessThan(idx("Rest and carve"));

      // Two dishes on one oven at overlapping times surface a conflict.
      await expect(
        page.getByRole("alert").filter({ hasText: /oven/i }).first()
      ).toBeVisible();
      await expect(page.getByText("conflict", { exact: true }).first()).toBeVisible();

      // The GraphQL contract agrees with what the UI rendered.
      const tl = await graphql<{
        eventTimeline: {
          warnings: string[];
          recipes: { name: string; steps: { stepNumber: number; conflicts: string[] }[] }[];
        };
      }>(
        request,
        token,
        `query ($id: ID!) {
          eventTimeline(foodEventId: $id) {
            warnings
            recipes { name steps { stepNumber conflicts } }
          }
        }`,
        { id: eventId }
      );
      const roast = tl.eventTimeline.recipes.find((r) => r.name === roastName);
      expect(roast?.steps.map((s) => s.stepNumber)).toEqual([1, 2, 3]);
      const conflicted = tl.eventTimeline.recipes.flatMap((r) =>
        r.steps.filter((s) => s.conflicts.length > 0)
      );
      expect(conflicted.length).toBeGreaterThanOrEqual(2);
    } finally {
      if (eventId) {
        await graphql(
          request,
          token,
          `mutation ($id: ID!) { deleteFoodEvent(id: $id) }`,
          { id: eventId }
        ).catch(() => undefined);
      }
      for (const id of [roastId, breadId]) {
        await graphql(
          request,
          token,
          `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
          { id }
        ).catch(() => undefined);
      }
    }
  });
});
