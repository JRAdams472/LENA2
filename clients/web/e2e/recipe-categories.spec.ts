import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

test.describe("recipe categories", () => {
  test("assign a category, filter the list, and outrank by engagement", async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const token = await mintToken(request);
    const tag = unique("cat");
    // Alphabetically the "other" recipe sorts first, so a viewed-first order
    // can only come from engagement ranking.
    const viewedName = `ZZZ Stew ${tag}`;
    const otherName = `AAA Soup ${tag}`;

    const create = (name: string) =>
      graphql<{ createRecipe: { id: string } }>(
        request,
        token,
        `mutation ($input: CreateRecipeInput!) {
          createRecipe(input: $input) { id }
        }`,
        {
          input: {
            name,
            description: "e2e categories",
            servings: 4,
            prepTimeMinutes: 10,
            cookTimeMinutes: 25,
            items: [],
            steps: [],
          },
        }
      );

    const viewed = (await create(viewedName)).createRecipe;
    const other = (await create(otherName)).createRecipe;

    try {
      // Opening the detail page records a recipe view for engagement ranking.
      await page.goto(`/recipes/${viewed.id}`);
      await expect(page.getByRole("heading", { name: viewedName })).toBeVisible();

      // Assign "Dinner" (Course — exclusive group) via the category picker.
      // The radio is server-controlled, so click and wait on the chip rather
      // than asserting the input state directly.
      await page.getByRole("radio", { name: "Dinner" }).click();
      await expect(page.getByText("Course: Dinner")).toBeVisible();

      // Filter the list by the same category: assigned recipe stays, the
      // uncategorized one drops out.
      await page.goto("/recipes");
      await page.getByLabel("Search").fill(tag);
      await page
        .getByRole("combobox", { name: "Course" })
        .click();
      await page.getByRole("option", { name: "Dinner" }).click();
      // Click closes? MUI multi-select keeps the dropdown open; dismiss it.
      await page.keyboard.press("Escape");
      await expect(
        page.getByRole("row", { name: new RegExp(viewedName) })
      ).toBeVisible();
      await expect(
        page.getByRole("row", { name: new RegExp(otherName) })
      ).toHaveCount(0);

      // Remove the filter, then verify engagement ranking puts the viewed
      // recipe above the alphabetically-earlier unviewed one.
      await page.getByRole("combobox", { name: "Course" }).click();
      await page.getByRole("option", { name: "Dinner" }).click();
      await page.keyboard.press("Escape");

      // Reload so the recorded view is reflected in ordering, then search for
      // both recipes (searching also records recipe_searched for the shared
      // tag, but a viewed recipe still ranks above searched-but-not-viewed).
      await page.reload();
      await page.getByLabel("Search").fill(tag);
      const firstRow = page.getByRole("row").nth(1); // row 0 is the header
      await expect(firstRow).toContainText(viewedName);
    } finally {
      for (const id of [viewed.id, other.id]) {
        await graphql(
          request,
          token,
          `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
          { id }
        );
      }
    }
  });
});
