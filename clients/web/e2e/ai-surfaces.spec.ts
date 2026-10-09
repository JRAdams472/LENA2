import { test, expect, type APIRequestContext } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

// Runs against LENA_AI_PROVIDER=mock (docker-compose.e2e.yml), whose
// canned JSON replies pick from the context the suggester sends.

function thisMonday(): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - ((d.getUTCDay() + 6) % 7));
  return d.toISOString().slice(0, 10);
}

function nextWeek(): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + 7);
  return d.toISOString().slice(0, 10);
}

async function seedRecipe(
  request: APIRequestContext,
  token: string,
  name: string,
  steps: { instruction: string; durationMinutes?: number; appliance?: string }[]
): Promise<string> {
  const res = await graphql<{ createRecipe: { id: string } }>(
    request,
    token,
    `mutation ($input: CreateRecipeInput!) { createRecipe(input: $input) { id } }`,
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

async function deleteRecipe(request: APIRequestContext, token: string, id: string) {
  await graphql(request, token, `mutation ($id: ID!) { deleteRecipe(id: $id) }`, {
    id,
  }).catch(() => undefined);
}

test.describe("AI surfaces (mock provider)", () => {
  test("Suggest Meals fills an open plan slot", async ({ page, request }) => {
    test.setTimeout(60_000);
    const token = await mintToken(request);
    const recipeId = await seedRecipe(request, token, unique("E2E Suggest Stew"), [
      { instruction: "Simmer" },
    ]);
    const plan = await graphql<{ createMealPlan: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateMealPlanInput!) { createMealPlan(input: $input) { id } }`,
      { input: { name: unique("E2E SuggestPlan"), weekStartDate: thisMonday() } }
    );
    const planId = plan.createMealPlan.id;
    const slotCount = async () => {
      const res = await graphql<{ mealPlan: { slots: { id: string }[] } }>(
        request,
        token,
        `query ($id: ID!) { mealPlan(id: $id) { slots { id } } }`,
        { id: planId }
      );
      return res.mealPlan.slots.length;
    };

    try {
      expect(await slotCount()).toBe(0);
      await page.goto(`/meal-plans/${planId}`);
      await page.getByRole("button", { name: "Suggest Meals" }).click();
      await expect(page.getByText("Suggested for your week")).toBeVisible();
      await expect(page.getByText("mock provider pick").first()).toBeVisible();
      await page.getByRole("button", { name: "Add to plan" }).first().click();
      await expect.poll(slotCount).toBe(1);
    } finally {
      await graphql(request, token, `mutation ($id: ID!) { deleteMealPlan(id: $id) }`, {
        id: planId,
      }).catch(() => undefined);
      await deleteRecipe(request, token, recipeId);
    }
  });

  test("Suggest Fixes proposes and applies a timeline fix", async ({ page, request }) => {
    test.setTimeout(60_000);
    const token = await mintToken(request);
    const eventDate = nextWeek();
    const roastId = await seedRecipe(request, token, unique("E2E Fix Roast"), [
      { instruction: "Roast", durationMinutes: 60, appliance: "oven" },
    ]);
    const breadId = await seedRecipe(request, token, unique("E2E Fix Bread"), [
      { instruction: "Bake", durationMinutes: 45, appliance: "oven" },
    ]);
    const ev = await graphql<{ createFoodEvent: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateFoodEventInput!) { createFoodEvent(input: $input) { id } }`,
      { input: { name: unique("E2E Fix Event"), eventDate } }
    );
    const eventId = ev.createFoodEvent.id;
    const conflicts = async () => {
      const res = await graphql<{ eventTimeline: { warnings: string[] } }>(
        request,
        token,
        `query ($id: ID!) { eventTimeline(foodEventId: $id) { warnings } }`,
        { id: eventId }
      );
      return res.eventTimeline.warnings.length;
    };

    try {
      for (const recipeId of [roastId, breadId]) {
        await graphql(
          request,
          token,
          `mutation ($input: AddEventRecipeInput!) { addEventRecipe(input: $input) { id } }`,
          {
            input: {
              foodEventId: eventId,
              recipeId,
              mealType: "dinner",
              targetTime: `${eventDate}T18:00:00Z`,
              servings: 4,
            },
          }
        );
      }
      expect(await conflicts()).toBeGreaterThan(0);

      await page.goto(`/events/${eventId}`);
      await page.getByRole("button", { name: "Generate Timeline" }).click();
      await page.getByRole("button", { name: "Suggest Fixes" }).click();
      const apply = page.getByRole("button", { name: "Apply" }).first();
      await expect(apply).toBeVisible();
      await apply.click();
      await expect.poll(conflicts).toBe(0);
    } finally {
      await graphql(request, token, `mutation ($id: ID!) { deleteFoodEvent(id: $id) }`, {
        id: eventId,
      }).catch(() => undefined);
      await deleteRecipe(request, token, roastId);
      await deleteRecipe(request, token, breadId);
    }
  });

  test("wine pairing is gated on a 21+ birthdate", async ({ page, request }) => {
    const token = await mintToken(request);
    const me = await graphql<{ me: { birthdate: string | null } }>(
      request,
      token,
      `{ me { birthdate } }`
    );
    const original = me.me.birthdate ?? "";
    const setBirthdate = (birthdate: string) =>
      graphql(
        request,
        token,
        `mutation ($input: UpdateProfileInput!) { updateMyProfile(input: $input) { id } }`,
        { input: { birthdate } }
      );
    const recipeId = await seedRecipe(request, token, unique("E2E Pairing Steak"), [
      { instruction: "Sear" },
    ]);
    const pairing = page.getByRole("button", { name: "Suggest wine pairing" });

    try {
      await setBirthdate("");
      await page.goto(`/recipes/${recipeId}`);
      await expect(page.getByRole("button", { name: "Tweak" }).first()).toBeVisible();
      await expect(pairing).toHaveCount(0);

      const underage = new Date();
      underage.setUTCFullYear(underage.getUTCFullYear() - 18);
      await setBirthdate(underage.toISOString().slice(0, 10));
      await page.reload();
      await expect(page.getByRole("button", { name: "Tweak" }).first()).toBeVisible();
      await expect(pairing).toHaveCount(0);

      await setBirthdate("1990-01-01");
      await page.reload();
      await expect(pairing).toBeVisible();
    } finally {
      await setBirthdate(original).catch(() => undefined);
      await deleteRecipe(request, token, recipeId);
    }
  });
});
