import { test, expect } from "@playwright/test";

test.describe("assistant", () => {
  // The e2e stack runs LENA_AI_PROVIDER=mock with a canned handler: the
  // first turn requests get_expiring_items (a real household tool call),
  // the turn after the tool result returns a fixed reply.
  test("asks a question and shows the answer plus tool trace", async ({
    page,
  }) => {
    await page.goto("/assistant");

    await expect(
      page.getByRole("heading", { name: "Ask Dot" })
    ).toBeVisible();

    await page.getByPlaceholder("Ask Dot…").fill("what is expiring soon?");
    await page.getByRole("button", { name: "Send" }).click();

    await expect(
      page.getByText("based on what's on hand", { exact: false })
    ).toBeVisible({ timeout: 30_000 });
    await expect(
      page.getByText(/looked up: get_expiring_items/)
    ).toBeVisible();
    await expect(page.getByText("what is expiring soon?")).toBeVisible();
  });

  test("navigates from the header", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "Ask Dot" }).first().click();
    await expect(page).toHaveURL(/\/assistant/);
    await expect(
      page.getByRole("heading", { name: "Ask Dot" })
    ).toBeVisible();
  });
});
