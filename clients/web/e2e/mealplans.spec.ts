import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

test.describe("meal plans", () => {
  test("create a plan, open it, generate a grocery list", async ({
    page,
    request,
  }) => {
    const token = await mintToken(request);
    const planName = unique("E2E Plan");

    await page.goto("/meal-plans");
    await page
      .getByRole("button", { name: "Create", exact: true })
      .click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Plan Name").fill(planName);
    await dialog.getByLabel("Week Start Date").fill("2026-09-07");
    await dialog.getByRole("button", { name: "Save" }).click();

    const row = page.getByRole("row", { name: new RegExp(planName) });
    await expect(row).toBeVisible();

    // Manage navigates to the detail page.
    await row.getByRole("link", { name: "Manage" }).click();
    await expect(page).toHaveURL(/\/meal-plans\/(\d+)/);
    const planId = page.url().match(/\/meal-plans\/(\d+)/)![1];

    try {
      // Generate a grocery list for the plan via the grocery-lists page.
      await page.goto("/grocery-lists");
      await page
        .getByRole("button", { name: "Create", exact: true })
        .click();
      const genDialog = page.getByRole("dialog");
      await genDialog.getByLabel("Meal Plan ID (optional)").fill(planId);
      await genDialog.getByRole("button", { name: "Save" }).click();

      // Success navigates to the new list's detail page.
      await expect(page).toHaveURL(/\/grocery-lists\/\d+/);
      await expect(
        page.getByRole("heading", { name: "Grocery List" })
      ).toBeVisible();

      // Add a manual item, check it off, then remove it.
      await page.getByLabel("Item Name").fill("E2E Eggs");
      await page.getByLabel("Qty").fill("2");
      await page.getByLabel("Unit").fill("ea");
      await page.getByRole("button", { name: "Add" }).click();
      await expect(page.getByText("E2E Eggs")).toBeVisible();

      const itemRow = page
        .locator("div")
        .filter({ has: page.getByRole("checkbox") })
        .filter({ hasText: "E2E Eggs" })
        .last();
      await itemRow.getByRole("checkbox").click();
      await expect(itemRow.getByRole("checkbox")).toBeChecked();

      await itemRow.locator("button").click();
      await expect(page.getByText("E2E Eggs")).toHaveCount(0);
    } finally {
      // There is no deleteGroceryList mutation, and grocery_list has an FK
      // to meal_plan, so the plan cannot be deleted once a list references
      // it. Attempt cleanup but tolerate the FK violation.
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteMealPlan(id: $id) }`,
        { id: planId }
      ).catch(() => undefined);
    }
  });

  test("edit and delete a meal plan", async ({ page }) => {
    const planName = unique("E2E PlanEdit");

    await page.goto("/meal-plans");
    await page
      .getByRole("button", { name: "Create", exact: true })
      .click();
    let dialog = page.getByRole("dialog");
    await dialog.getByLabel("Plan Name").fill(planName);
    await dialog.getByLabel("Week Start Date").fill("2026-09-14");
    await dialog.getByRole("button", { name: "Save" }).click();

    const row = page.getByRole("row", { name: new RegExp(planName) });
    await expect(row).toBeVisible();

    await row.locator("button").first().click();
    dialog = page.getByRole("dialog");
    const renamed = `${planName} v2`;
    await dialog.getByLabel("Plan Name").fill(renamed);
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(page.getByText(renamed)).toBeVisible();

    page.once("dialog", (d) => d.accept());
    const row2 = page.getByRole("row", { name: new RegExp(renamed) });
    await row2.locator("button").nth(1).click();
    await expect(page.getByText(renamed)).toHaveCount(0);
  });
});
