import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

test.describe("recipe detail", () => {
  test("view a recipe and manage its steps", async ({ page, request }) => {
    const token = await mintToken(request);
    const name = unique("E2E RecipeDetail");

    const created = await graphql<{ createRecipe: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateRecipeInput!) {
        createRecipe(input: $input) { id }
      }`,
      {
        input: {
          name,
          description: "created by e2e",
          servings: 4,
          prepTimeMinutes: 10,
          cookTimeMinutes: 25,
          items: [],
          steps: [],
        },
      }
    );
    const recipeId = created.createRecipe.id;

    try {
      await page.goto(`/recipes/${recipeId}`);

      // Header fields.
      await expect(
        page.getByRole("heading", { name })
      ).toBeVisible();
      await expect(page.getByText("created by e2e")).toBeVisible();
      await expect(page.getByText("Servings: 4")).toBeVisible();
      await expect(
        page.getByRole("heading", { name: "Ingredients" })
      ).toBeVisible();
      await expect(
        page.getByRole("heading", { name: "Steps" })
      ).toBeVisible();
      await expect(page.getByText("No steps")).toBeVisible();

      // Add a step.
      const step1 = unique("E2E step one");
      await page.getByLabel("Step Number").fill("1");
      await page.getByLabel("Instruction").fill(step1);
      await page.getByRole("button", { name: "Add Step" }).click();
      await expect(page.getByText(step1)).toBeVisible();
      await expect(page.getByText("1.")).toBeVisible();

      // Edit the step.
      await page.getByRole("button", { name: "Edit" }).click();
      const step1v2 = `${step1} v2`;
      await page.getByLabel("Instruction").fill(step1v2);
      await page
        .getByRole("button", { name: "Save Step" })
        .click();
      await expect(page.getByText(step1v2)).toBeVisible();

      // Delete the step.
      page.once("dialog", (d) => d.accept());
      await page.getByRole("button", { name: "Delete" }).click();
      await expect(page.getByText(step1v2)).toHaveCount(0);
      await expect(page.getByText("No steps")).toBeVisible();
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
        { id: recipeId }
      );
    }
  });

  test("view and remove an ingredient", async ({ page, request }) => {
    const token = await mintToken(request);
    const catName = unique("E2E RecipeCat");
    const itemName = unique("E2E Ingredient");
    const recipeName = unique("E2E RecipeItems");

    const cat = await graphql<{ createCategory: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateCategoryInput!) {
        createCategory(input: $input) { id }
      }`,
      { input: { name: catName, description: null } }
    );
    const item = await graphql<{ createItem: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateItemInput!) {
        createItem(input: $input) { id }
      }`,
      {
        input: {
          name: itemName,
          categoryId: cat.createCategory.id,
          unit: "ea",
        },
      }
    );
    // Seed the recipe with the ingredient via the API — the Item
    // autocomplete's debounced search is too timing-sensitive for e2e.
    const recipe = await graphql<{ createRecipe: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateRecipeInput!) {
        createRecipe(input: $input) { id }
      }`,
      {
        input: {
          name: recipeName,
          items: [
            {
              itemId: item.createItem.id,
              quantity: 2,
              unit: "ea",
              isOptional: false,
            },
          ],
          steps: [],
        },
      }
    );
    const recipeId = recipe.createRecipe.id;

    try {
      await page.goto(`/recipes/${recipeId}`);
      await expect(
        page.getByRole("heading", { name: recipeName })
      ).toBeVisible();

      // The ingredient row renders in the items table.
      const row = page.getByRole("row", { name: new RegExp(itemName) });
      await expect(row).toBeVisible();
      await expect(row.getByText("2 ea")).toBeVisible();

      // Remove it.
      await row.getByRole("button", { name: "Remove" }).click();
      await expect(page.getByText(itemName)).toHaveCount(0);
      await expect(page.getByText("No ingredients")).toBeVisible();
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
        { id: recipeId }
      );
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteItem(id: $id) }`,
        { id: item.createItem.id }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteCategory(id: $id) }`,
        { id: cat.createCategory.id }
      ).catch(() => undefined);
    }
  });
});
