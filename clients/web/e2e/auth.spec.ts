import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique, SECOND_USER } from "./helpers";

// These tests run without the stored auth state.
test.use({ storageState: { cookies: [], origins: [] } });

test.describe("auth", () => {
  test("unauthenticated users see the login screen", async ({ page }) => {
    await page.goto("/");
    await expect(
      page.getByText(
        "Sign in to manage inventory, recipes, and meal plans."
      )
    ).toBeVisible();
  });

  test("invalid tokens are rejected by the API", async ({ request }) => {
    const res = await request.post(
      `${process.env.E2E_BASE_URL ?? "http://localhost"}/graphql`,
      {
        headers: { Authorization: "Bearer not.a.token" },
        data: { query: "{ __typename }" },
      }
    );
    expect(res.status()).toBe(401);
  });

  test("non-admin users are rejected from catalog mutations", async ({
    request,
  }) => {
    // e2e@example.com is seeded admin via LENA_ADMIN_EMAILS; the second
    // user stays a member and must be forbidden from shared-catalog writes.
    const tokenB = await mintToken(request, SECOND_USER);
    await expect(
      graphql(
        request,
        tokenB,
        `mutation ($input: CreateCategoryInput!) {
          createCategory(input: $input) { id }
        }`,
        { input: { name: unique("Forbidden category") } }
      )
    ).rejects.toThrow(/forbidden/i);
  });

  test("users cannot see each other's meal plans", async ({
    request,
    browser,
  }) => {
    // The shared storage state holds the primary user's token.
    const context = await browser.newContext();
    const tokenA = await mintToken(request);
    const tokenB = await mintToken(request, SECOND_USER);
    await context.close();

    const planName = unique("Isolation plan");
    const created = await graphql<{ createMealPlan: { id: string } }>(
      request,
      tokenA,
      `mutation ($input: CreateMealPlanInput!) {
        createMealPlan(input: $input) { id }
      }`,
      { input: { name: planName, weekStartDate: "2026-09-07", weekStartDayOfWeek: 1 } }
    );
    const planId = created.createMealPlan.id;

    try {
      // User B must not see user A's plan in the list or by id.
      const listB = await graphql<{
        mealPlans: { items: { id: string; name: string }[] };
      }>(
        request,
        tokenB,
        `query { mealPlans(page: 1, pageSize: 200) { items { id name } } }`
      );
      expect(listB.mealPlans.items.map((p) => p.id)).not.toContain(planId);

      // Fetching another user's plan by id fails (no rows for this user).
      await expect(
        graphql(
          request,
          tokenB,
          `query ($id: ID!) { mealPlan(id: $id) { id } }`,
          { id: planId }
        )
      ).rejects.toThrow();
    } finally {
      await graphql(
        request,
        tokenA,
        `mutation ($id: ID!) { deleteMealPlan(id: $id) }`,
        { id: planId }
      );
    }
  });
});
