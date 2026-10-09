import { test, expect, type APIRequestContext } from "@playwright/test";
import { graphql, mintToken, unique, SECOND_USER } from "./helpers";

async function submitPending(
  request: APIRequestContext,
  token: string,
  categoryId: string,
  name: string
): Promise<string> {
  const res = await graphql<{ submitItem: { id: string } }>(
    request,
    token,
    `mutation ($input: CreateItemInput!) { submitItem(input: $input) { id } }`,
    { input: { name, categoryId, unit: "ea" } }
  );
  return res.submitItem.id;
}

test.describe("admin review", () => {
  test("approve and reject member-submitted items on /items/pending", async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const adminToken = await mintToken(request);
    const memberToken = await mintToken(request, SECOND_USER);
    const approveName = unique("E2E Pending Approve");
    const rejectName = unique("E2E Pending Reject");
    const cat = await graphql<{ createCategory: { id: string } }>(
      request,
      adminToken,
      `mutation ($input: CreateCategoryInput!) { createCategory(input: $input) { id } }`,
      { input: { name: unique("E2E PendingCat"), description: null } }
    );
    const ids: string[] = [];

    try {
      ids.push(await submitPending(request, memberToken, cat.createCategory.id, approveName));
      ids.push(await submitPending(request, memberToken, cat.createCategory.id, rejectName));

      await page.goto("/items/pending");
      for (const [name, action] of [
        [approveName, "Approve"],
        [rejectName, "Reject"],
      ] as const) {
        const row = page.getByRole("row", { name: new RegExp(name) });
        await expect(row).toBeVisible();
        await row.getByRole("button", { name: action }).click();
        await page.getByRole("dialog").getByRole("button", { name: "Confirm" }).click();
        await expect(row).toHaveCount(0);
      }

      const pending = await graphql<{ pendingItems: { items: { id: string }[] } }>(
        request,
        adminToken,
        `{ pendingItems(pageSize: 100) { items { id } } }`
      );
      expect(pending.pendingItems.items.map((i) => i.id)).not.toContain(ids[0]);
      expect(pending.pendingItems.items.map((i) => i.id)).not.toContain(ids[1]);

    } finally {
      for (const id of ids) {
        await graphql(request, adminToken, `mutation ($id: ID!) { deleteItem(id: $id) }`, {
          id,
        }).catch(() => undefined);
      }
      await graphql(
        request,
        adminToken,
        `mutation ($id: ID!) { deleteCategory(id: $id) }`,
        { id: cat.createCategory.id }
      ).catch(() => undefined);
    }
  });

  test("accept an AI allergen suggestion on /inventory/allergen-suggestions", async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const token = await mintToken(request);
    const ingredientName = unique("E2E Suggest Flour");
    const recipeName = unique("E2E Suggest Recipe");
    // An active allergen guarantees the registry the suggester sees is
    // non-empty; the mock provider flags the first unflagged line.
    const allergen = await graphql<{ createAllergen: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateAllergenInput!) { createAllergen(input: $input) { id } }`,
      { input: { name: unique("E2E Suggest Allergen"), description: "e2e" } }
    );
    const ingredient = await graphql<{ createIngredient: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateIngredientInput!) { createIngredient(input: $input) { id } }`,
      { input: { name: ingredientName, defaultUnit: "g" } }
    );
    const ingredientId = ingredient.createIngredient.id;
    const recipe = await graphql<{ createRecipe: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateRecipeInput!) { createRecipe(input: $input) { id } }`,
      {
        input: {
          name: recipeName,
          servings: 2,
          items: [{ ingredientId, quantity: 100, unit: "g" }],
          steps: [{ stepNumber: 1, instruction: "Sift the flour" }],
        },
      }
    );
    const recipeId = recipe.createRecipe.id;
    let acceptedAllergenId: string | null = null;

    try {
      await page.goto("/inventory/allergen-suggestions");
      await page.getByLabel("Recipe").fill(recipeName);
      await page.getByRole("option", { name: recipeName }).click();
      await page.getByRole("button", { name: "Suggest flags" }).click();

      const row = page.getByRole("row").filter({ hasText: ingredientName });
      await expect(row).toBeVisible();
      await expect(row).toContainText("contains");
      await expect(row).toContainText("pending");
      await row.getByRole("button", { name: "Accept" }).click();
      // Accepted rows leave the default (pending) review queue.
      await expect(row).toHaveCount(0);

      const res = await graphql<{
        allergenSuggestions: {
          ingredientId: string | null;
          status: string;
          allergen: { id: string };
        }[];
      }>(
        request,
        token,
        `{ allergenSuggestions(status: "accepted") { ingredientId status allergen { id } } }`
      );
      const accepted = res.allergenSuggestions.find((s) => s.ingredientId === ingredientId);
      expect(accepted?.status).toBe("accepted");
      acceptedAllergenId = accepted?.allergen.id ?? null;
    } finally {
      if (acceptedAllergenId) {
        await graphql(
          request,
          token,
          `mutation ($ing: ID!, $al: ID!) {
            setIngredientAllergen(ingredientId: $ing, allergenId: $al, kind: null)
          }`,
          { ing: ingredientId, al: acceptedAllergenId }
        ).catch(() => undefined);
      }
      await graphql(request, token, `mutation ($id: ID!) { deleteRecipe(id: $id) }`, {
        id: recipeId,
      }).catch(() => undefined);
      await graphql(request, token, `mutation ($id: ID!) { deleteIngredient(id: $id) }`, {
        id: ingredientId,
      }).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { updateAllergen(id: $id, input: { isActive: false }) { id } }`,
        { id: allergen.createAllergen.id }
      ).catch(() => undefined);
    }
  });

  test("members are turned away from admin review pages", async ({
    browser,
    request,
    baseURL,
  }) => {
    const memberToken = await mintToken(request, SECOND_USER);
    const ctx = await browser.newContext({ baseURL });
    await ctx.addInitScript(
      (t) => window.localStorage.setItem("lena_id_token", t),
      memberToken
    );
    const page = await ctx.newPage();
    try {
      await page.goto("/items/pending");
      await expect(
        page.getByText("Forbidden: this page requires the admin role.")
      ).toBeVisible();
      await expect(page.getByRole("button", { name: "Approve" })).toHaveCount(0);
      for (const query of [
        `{ pendingItems { items { id } } }`,
        `{ allergenSuggestions { id } }`,
      ]) {
        await expect(graphql(request, memberToken, query)).rejects.toThrow(/forbidden/i);
      }
    } finally {
      await ctx.close();
    }
  });

  // FINDING: /inventory/allergen-suggestions has no client-side admin
  // guard (unlike /items/pending), so members get the full review UI
  // backed by forbidden API calls. Remove test.fail() once it's gated.
  test("members see the forbidden notice on allergen suggestions", async ({
    browser,
    request,
    baseURL,
  }) => {
    test.fail();
    const memberToken = await mintToken(request, SECOND_USER);
    const ctx = await browser.newContext({ baseURL });
    await ctx.addInitScript(
      (t) => window.localStorage.setItem("lena_id_token", t),
      memberToken
    );
    const page = await ctx.newPage();
    try {
      await page.goto("/inventory/allergen-suggestions");
      await expect(page.getByText(SECOND_USER.email).first()).toBeVisible();
      await expect(page.getByRole("button", { name: "Suggest flags" })).toHaveCount(0, {
        timeout: 5_000,
      });
    } finally {
      await ctx.close();
    }
  });
});
