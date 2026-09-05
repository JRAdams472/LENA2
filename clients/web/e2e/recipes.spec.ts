import { test, expect } from "@playwright/test";
import { unique } from "./helpers";

test.describe("recipes", () => {
  test("create, edit, and delete a recipe", async ({ page }) => {
    const name = unique("E2E Recipe");

    await page.goto("/recipes");
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
    await row.locator("button").first().click();
    dialog = page.getByRole("dialog");
    await dialog.getByLabel("Description").fill("updated by e2e");
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(page.getByText("updated by e2e")).toBeVisible();

    // Manage link navigates to the detail page.
    await row.getByRole("link", { name: "Manage" }).click();
    await expect(page).toHaveURL(/\/recipes\/\d+/);

    // Back to the list, delete the recipe.
    await page.goto("/recipes");
    await page.getByLabel("Search").fill(name);
    const row2 = page.getByRole("row", { name: new RegExp(name) });
    await expect(row2).toBeVisible();
    page.once("dialog", (d) => d.accept());
    await row2.locator("button").nth(1).click();
    await expect(page.getByText(name)).toHaveCount(0);
  });
});
