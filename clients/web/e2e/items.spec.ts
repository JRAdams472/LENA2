import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

test.describe("items", () => {
  test("create, search, favorite, and delete an item", async ({
    page,
    request,
  }) => {
    const token = await mintToken(request);

    // An item requires a category; create one via the API.
    const catName = unique("E2E ItemCat");
    const cat = await graphql<{ createCategory: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateCategoryInput!) {
        createCategory(input: $input) { id }
      }`,
      { input: { name: catName, description: null } }
    );
    const categoryId = cat.createCategory.id;
    const itemName = unique("E2E Item");

    try {
      await page.goto("/inventory/items");
      await page
        .getByRole("button", { name: "Create", exact: true })
        .click();
      const dialog = page.getByRole("dialog");
      await dialog.getByLabel("Name").fill(itemName);
      await dialog.getByLabel("Category ID").fill(categoryId);
      await dialog.getByLabel("Unit").fill("ea");
      await dialog.getByRole("button", { name: "Save" }).click();

      // Find it via the search box.
      await page.getByLabel("Search").fill(itemName);
      await expect(page.getByText(itemName).first()).toBeVisible();

      // Toggle favorite on the row.
      const row = page.getByRole("row", { name: new RegExp(itemName) });
      await row.getByRole("button", { name: "Fav" }).click();
      await expect(
        row.getByRole("button", { name: "Unfav" })
      ).toBeVisible();

      // Delete it.
      page.once("dialog", (d) => d.accept());
      await row.locator("button").nth(1).click();
      await expect(page.getByText(itemName)).toHaveCount(0);
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteCategory(id: $id) }`,
        { id: categoryId }
      );
    }
  });
});
