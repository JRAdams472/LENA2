import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

test.describe("recipes", () => {
  test("create, edit, and delete a recipe", async ({ page }) => {
    const name = unique("E2E Recipe");

    await page.goto("/member/recipes");
    await page
      .getByRole("button", { name: "Create", exact: true })
      .click();
    let dialog = page.getByRole("dialog");
    await dialog.getByLabel("Name").fill(name);
    await dialog.getByLabel("Description").fill("made by e2e");
    await dialog.getByLabel("Servings").fill("4");
    await dialog.getByLabel("Prep Time").fill("10");
    await dialog.getByLabel("Cook Time").fill("25");
    await dialog.getByRole("button", { name: "Save" }).click();

    // Search narrows the table to the new recipe.
    await page.getByLabel("Search").fill(name);
    const row = page.getByRole("row", { name: new RegExp(name) });
    await expect(row).toBeVisible();

    // Edit the description.
    await row.getByTestId("row-edit-button").click();
    dialog = page.getByRole("dialog");
    await dialog.getByLabel("Description").fill("updated by e2e");
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(page.getByText("updated by e2e").first()).toBeVisible();

    // Manage link navigates to the detail page.
    await row.getByRole("link", { name: "Manage" }).click();
    await expect(page).toHaveURL(/\/recipes\/\d+/);

    // Back to the list, delete the recipe.
    await page.goto("/member/recipes");
    await page.getByLabel("Search").fill(name);
    const row2 = page.getByRole("row", { name: new RegExp(name) });
    await expect(row2).toBeVisible();
    page.once("dialog", (d) => d.accept());
    await row2.getByTestId("row-delete-button").click();
    await expect(page.getByText(name)).toHaveCount(0);
  });

  test("preview shows ingredients and steps without the editor", async ({
    page,
    request,
  }) => {
    const token = await mintToken(request);
    const ingredientName = unique("E2E Preview Flour");
    const recipeName = unique("E2E Preview Recipe");

    const ingredient = await graphql<{ createIngredient: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateIngredientInput!) { createIngredient(input: $input) { id } }`,
      { input: { name: ingredientName, defaultUnit: "g" } }
    );
    await graphql(
      request,
      token,
      `mutation ($input: CreateRecipeInput!) { createRecipe(input: $input) { id } }`,
      {
        input: {
          name: recipeName,
          description: "peek at me",
          servings: 2,
          items: [
            { ingredientId: ingredient.createIngredient.id, quantity: 100, unit: "g" },
          ],
          steps: [
            { stepNumber: 1, instruction: "Sift the flour", durationMinutes: 5 },
            { stepNumber: 2, instruction: "Fold and bake" },
          ],
        },
      }
    );

    await page.goto("/member/recipes");
    await page.getByLabel("Search").fill(recipeName);
    const row = page.getByRole("row", { name: new RegExp(recipeName) });
    await expect(row).toBeVisible();

    // Clicking the recipe name opens a read-only preview, not the editor.
    await row.getByRole("button", { name: recipeName }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("Ingredients")).toBeVisible();
    await expect(dialog.getByText(new RegExp(ingredientName))).toBeVisible();
    await expect(dialog.getByText("1. Sift the flour")).toBeVisible();
    await expect(dialog.getByText("5 min")).toBeVisible();
    await expect(dialog.getByText("2. Fold and bake")).toBeVisible();

    // Closing returns to the unchanged table; the description cell opens
    // the same preview.
    await dialog.getByRole("button", { name: "Close" }).click();
    await expect(dialog).toHaveCount(0);
    await row.getByRole("button", { name: "peek at me" }).click();
    await expect(
      page.getByRole("dialog").getByText("Ingredients")
    ).toBeVisible();
  });
});
