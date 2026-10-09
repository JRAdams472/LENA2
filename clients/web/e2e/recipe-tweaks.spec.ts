import { test, expect, type APIRequestContext } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

async function createIngredient(
  request: APIRequestContext,
  token: string,
  name: string
): Promise<string> {
  const res = await graphql<{ createIngredient: { id: string } }>(
    request,
    token,
    `mutation ($input: CreateIngredientInput!) {
      createIngredient(input: $input) { id }
    }`,
    { input: { name, defaultUnit: "g" } }
  );
  return res.createIngredient.id;
}

test.describe("recipe household tweaks", () => {
  test("swap a line, toggle versions, then review a stale tweak", async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const token = await mintToken(request);
    const recipeName = unique("E2E Tweak Recipe");
    const butter = unique("E2E Tweak Butter");
    const oil = unique("E2E Tweak Oil");
    const butterId = await createIngredient(request, token, butter);
    const oilId = await createIngredient(request, token, oil);
    const recipeInput = (stepText: string) => ({
      name: recipeName,
      servings: 2,
      items: [{ ingredientId: butterId, quantity: 30, unit: "g" }],
      steps: [{ stepNumber: 1, instruction: stepText }],
    });
    const recipe = await graphql<{ createRecipe: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateRecipeInput!) { createRecipe(input: $input) { id } }`,
      { input: recipeInput("Melt the fat") }
    );
    const recipeId = recipe.createRecipe.id;

    try {
      await page.goto(`/recipes/${recipeId}`);
      const line = page.getByRole("row").filter({ hasText: butter });
      await expect(line).toBeVisible();

      // Swap the butter line for oil through the tweak editor.
      await line.getByRole("button", { name: "Tweak" }).click();
      await page.getByLabel("Swap").check();
      // The inline editor opens prefilled with the line's ingredient.
      const picker = page.locator(`input[role="combobox"][value="${butter}"]`);
      await picker.fill(oil);
      await page.getByRole("option", { name: oil }).click();
      await page.getByRole("button", { name: "Apply tweak" }).click();
      // Applied tweaks are staged in the editor until saved.
      await expect(page.getByText(`Swap ${butter} for ${oil}`)).toBeVisible();
      await page.getByRole("button", { name: "Save tweaks" }).click();

      // Household view shows the swap with its badge.
      const swapped = page.getByRole("row").filter({ hasText: oil });
      await expect(swapped).toBeVisible();
      await expect(swapped.getByText("Swapped")).toBeVisible();

      // Original view hides the tweak.
      await page.getByRole("button", { name: "Original recipe" }).click();
      await expect(page.getByRole("row").filter({ hasText: butter })).toBeVisible();
      await expect(page.getByRole("row").filter({ hasText: oil })).toHaveCount(0);
      await expect(page.getByText("Swapped")).toHaveCount(0);
      await page.getByRole("button", { name: "Household version" }).click();
      await expect(page.getByRole("row").filter({ hasText: oil })).toBeVisible();

      // Editing the canonical recipe marks the tweak stale.
      await graphql(
        request,
        token,
        `mutation ($id: ID!, $input: CreateRecipeInput!) {
          updateRecipe(id: $id, input: $input) { id }
        }`,
        { id: recipeId, input: recipeInput("Melt the fat over low heat") }
      );
      await page.reload();
      const stale = page
        .getByRole("alert")
        .filter({ hasText: "The original recipe changed after these tweaks were saved." });
      await expect(stale).toBeVisible();
      await stale.getByRole("button", { name: "Mark reviewed" }).click();
      await expect(stale).toBeHidden();

      // Acknowledgement is server-side: it stays cleared on reload.
      await page.reload();
      await expect(page.getByRole("heading", { name: recipeName })).toBeVisible();
      await expect(
        page.getByText("The original recipe changed after these tweaks were saved.")
      ).toHaveCount(0);
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { clearRecipeDelta(recipeId: $id) }`,
        { id: recipeId }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
        { id: recipeId }
      ).catch(() => undefined);
      for (const id of [butterId, oilId]) {
        await graphql(
          request,
          token,
          `mutation ($id: ID!) { deleteIngredient(id: $id) }`,
          { id }
        ).catch(() => undefined);
      }
    }
  });

  // FINDING: updateRecipe replaces every recipe line and step
  // (RecipeService.UpdateRecipeWithChildren), so any canonical edit —
  // even rewording a step — orphans all household line tweaks. Remove
  // test.fail() once unchanged lines keep their anchors.
  test("a line swap survives an unrelated canonical edit", async ({ page, request }) => {
    test.fail();
    const token = await mintToken(request);
    const recipeName = unique("E2E Tweak Anchor");
    const butter = unique("E2E Anchor Butter");
    const oil = unique("E2E Anchor Oil");
    const butterId = await createIngredient(request, token, butter);
    const oilId = await createIngredient(request, token, oil);
    const recipeInput = (stepText: string) => ({
      name: recipeName,
      servings: 2,
      items: [{ ingredientId: butterId, quantity: 30, unit: "g" }],
      steps: [{ stepNumber: 1, instruction: stepText }],
    });
    const recipe = await graphql<{ createRecipe: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateRecipeInput!) { createRecipe(input: $input) { id } }`,
      { input: recipeInput("Melt the fat") }
    );
    const recipeId = recipe.createRecipe.id;
    try {
      const detail = await graphql<{ recipe: { items: { id: string }[] } }>(
        request,
        token,
        `query ($id: ID!) { recipe(id: $id) { items { id } } }`,
        { id: recipeId }
      );
      await graphql(
        request,
        token,
        `mutation ($id: ID!, $items: [RecipeDeltaItemInput!]!) {
          setRecipeDelta(recipeId: $id, items: $items, steps: []) { stale }
        }`,
        {
          id: recipeId,
          items: [
            {
              kind: "substitute",
              recipeItemId: detail.recipe.items[0].id,
              ingredientId: oilId,
            },
          ],
        }
      );
      // Only the step wording changes; the butter line is untouched.
      await graphql(
        request,
        token,
        `mutation ($id: ID!, $input: CreateRecipeInput!) {
          updateRecipe(id: $id, input: $input) { id }
        }`,
        { id: recipeId, input: recipeInput("Melt the fat over low heat") }
      );
      await page.goto(`/recipes/${recipeId}`);
      await expect(page.getByRole("heading", { name: recipeName })).toBeVisible();
      await expect(page.getByRole("row").filter({ hasText: oil })).toBeVisible();
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { clearRecipeDelta(recipeId: $id) }`,
        { id: recipeId }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
        { id: recipeId }
      ).catch(() => undefined);
      for (const id of [butterId, oilId]) {
        await graphql(
          request,
          token,
          `mutation ($id: ID!) { deleteIngredient(id: $id) }`,
          { id }
        ).catch(() => undefined);
      }
    }
  });
});
