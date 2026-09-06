import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

test.describe("meal plan detail", () => {
  test("view the weekly grid and manage a slot", async ({
    page,
    request,
  }) => {
    const token = await mintToken(request);
    const planName = unique("E2E PlanDetail");

    const created = await graphql<{ createMealPlan: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateMealPlanInput!) {
        createMealPlan(input: $input) { id }
      }`,
      {
        input: {
          name: planName,
          weekStartDate: "2026-09-07",
          weekStartDayOfWeek: 1,
        },
      }
    );
    const planId = created.createMealPlan.id;

    try {
      await page.goto(`/meal-plans/${planId}`);

      // Header and weekly grid.
      await expect(
        page.getByRole("heading", { name: planName })
      ).toBeVisible();
      await expect(
        page.getByText(/Week starting 2026-09-07/)
      ).toBeVisible();
      await expect(
        page.getByRole("heading", { name: "Weekly Grid" })
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Generate Grocery List" })
      ).toBeVisible();

      // Empty cells show "Blank"; open the first one (Sun - Breakfast).
      // The slot editor is a custom overlay, not a role=dialog.
      await page.getByText("Blank", { exact: true }).first().click();
      await expect(page.getByText("Sun - Breakfast")).toBeVisible();

      // Save a slot with a replacement note.
      const note = unique("E2E note");
      await page.getByLabel("Replacement Note").fill(note);
      await page.getByRole("button", { name: "Save" }).click();
      await expect(page.getByText("Sun - Breakfast")).toHaveCount(0);
      await expect(page.getByText(note)).toBeVisible();

      // Reopen the same cell and remove the slot.
      await page.getByText(note).click();
      await expect(page.getByText("Sun - Breakfast")).toBeVisible();
      await page
        .getByRole("button", { name: "Remove Slot" })
        .click();
      await expect(page.getByText(note)).toHaveCount(0);
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteMealPlan(id: $id) }`,
        { id: planId }
      ).catch(() => undefined);
    }
  });
});
