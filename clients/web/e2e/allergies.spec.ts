import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique, PRIMARY_USER } from "./helpers";

test.describe("allergy warnings", () => {
  test("a member allergy to a flagged ingredient warns on the recipe list and detail", async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const token = await mintToken(request);
    const allergenName = unique("E2E Allergen");
    const ingredientName = unique("E2E Allergy Nut");
    const recipeName = unique("E2E Allergy Recipe");

    const allergen = await graphql<{ createAllergen: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateAllergenInput!) {
        createAllergen(input: $input) { id }
      }`,
      { input: { name: allergenName, description: "e2e" } }
    );
    const allergenId = allergen.createAllergen.id;
    const ingredient = await graphql<{ createIngredient: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateIngredientInput!) {
        createIngredient(input: $input) { id }
      }`,
      { input: { name: ingredientName, defaultUnit: "g" } }
    );
    const ingredientId = ingredient.createIngredient.id;
    const recipe = await graphql<{ createRecipe: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateRecipeInput!) {
        createRecipe(input: $input) { id }
      }`,
      {
        input: {
          name: recipeName,
          servings: 2,
          items: [{ ingredientId, quantity: 50, unit: "g" }],
          steps: [{ stepNumber: 1, instruction: "Toast the nuts" }],
        },
      }
    );
    const recipeId = recipe.createRecipe.id;

    try {
      // Without a member record or a flag, nothing warns.
      await page.goto(`/recipes/${recipeId}`);
      await expect(page.getByRole("heading", { name: recipeName })).toBeVisible();
      await expect(page.getByTestId("allergy-warning-alert")).toHaveCount(0);

      // The member's allergy record plus a `contains` flag on the line.
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { setMyAllergy(allergenId: $id, kind: allergy, on: true) }`,
        { id: allergenId }
      );
      await graphql(
        request,
        token,
        `mutation ($ing: ID!, $al: ID!) {
          setIngredientAllergen(ingredientId: $ing, allergenId: $al, kind: contains)
        }`,
        { ing: ingredientId, al: allergenId }
      );

      // Recipe list row carries the warning chip.
      await page.goto("/recipes");
      await page.getByLabel("Search").fill(recipeName);
      const row = page.getByRole("row", { name: new RegExp(recipeName) });
      await expect(row.getByTestId("allergy-warning-chip")).toHaveText(
        "1 allergy warning"
      );

      // Detail alert names the member, allergen, and flag kind.
      await page.goto(`/recipes/${recipeId}`);
      const alert = page.getByTestId("allergy-warning-alert");
      await expect(alert).toBeVisible();
      await expect(alert).toContainText(
        `${PRIMARY_USER.name} — ${allergenName} (allergy; contains)`
      );
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { setMyAllergy(allergenId: $id, kind: allergy, on: false) }`,
        { id: allergenId }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($ing: ID!, $al: ID!) {
          setIngredientAllergen(ingredientId: $ing, allergenId: $al, kind: null)
        }`,
        { ing: ingredientId, al: allergenId }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
        { id: recipeId }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteIngredient(id: $id) }`,
        { id: ingredientId }
      ).catch(() => undefined);
      // Allergens have no delete; retire the row instead.
      await graphql(
        request,
        token,
        `mutation ($id: ID!) {
          updateAllergen(id: $id, input: { isActive: false }) { id }
        }`,
        { id: allergenId }
      ).catch(() => undefined);
    }
  });
});
