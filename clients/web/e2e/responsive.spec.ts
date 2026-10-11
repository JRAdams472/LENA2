import { test, expect } from "@playwright/test";
import { randomInt } from "node:crypto";

// Runs under the "mobile" Playwright project (375px viewport). Covers the
// LEN-30 acceptance bar: navigate sections, lists render, CRUD dialogs
// open and submit, grocery check-off works, nothing forces horizontal
// scroll, and the app-switch prompt shows once per device.
test.describe("phone-sized member experience", () => {
  test("navigates member sections via the hamburger drawer", async ({
    page,
  }) => {
    await page.goto("/member");
    await page.getByRole("button", { name: "Open navigation" }).click();
    await page.getByRole("button", { name: "Meal Planning" }).click();
    await page.getByRole("link", { name: "Grocery Lists" }).click();
    await expect(page).toHaveURL(/\/member\/grocery-lists/);
    await expect(
      page.getByRole("heading", { name: /grocery/i }).first()
    ).toBeVisible();
  });

  test("pages fit the viewport without horizontal scroll", async ({
    page,
  }) => {
    for (const path of [
      "/member",
      "/member/grocery-lists",
      "/member/meal-plans",
      "/member/recipes",
    ]) {
      await page.goto(path);
      await page.waitForLoadState("domcontentloaded");
      const { scroll, client } = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth,
        client: document.documentElement.clientWidth,
      }));
      expect(scroll, `${path} overflows the viewport`).toBeLessThanOrEqual(
        client + 1
      );
    }
  });

  test("CRUD dialog opens full-screen and submits", async ({ page }) => {
    await page.goto("/member/inventory/categories");
    await page.getByRole("button", { name: "Create" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    // fullScreen below sm — the paper fills the viewport.
    await expect(dialog.locator(".MuiDialog-paper")).toHaveClass(
      /MuiDialog-paperFullScreen/
    );

    const name = `E2E Mobile Category ${randomInt(10000, 99999)}`;
    await dialog.getByLabel("Name").fill(name);
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(page.getByText(name).first()).toBeVisible({
      timeout: 15000,
    });
  });

  test("checks off a grocery item from a list detail", async ({ page }) => {
    await page.goto("/member/grocery-lists");
    await page
      .getByRole("link", { name: /grocery/i })
      .or(page.locator("a[href*='/member/grocery-lists/']"))
      .first()
      .click();
    const box = page.getByRole("checkbox").first();
    await expect(box).toBeVisible({ timeout: 15000 });
    const wasChecked = await box.isChecked();
    await box.click();
    // The check toggles through a mutation + refetch — wait for the
    // flipped state rather than asserting instantly.
    await expect(box).toBeChecked({ checked: !wasChecked, timeout: 15000 });
  });

  test("app-switch prompt shows once per device and stays dismissed", async ({
    page,
  }) => {
    await page.goto("/member");
    await page.evaluate(() =>
      window.localStorage.removeItem("lena_app_prompt_dismissed")
    );
    await page.reload();
    const prompt = page.getByTestId("app-switch-prompt");
    await expect(prompt).toBeVisible();
    await page.getByRole("button", { name: "Dismiss" }).click();
    await expect(prompt).not.toBeVisible();
    await page.reload();
    await expect(prompt).not.toBeVisible();
  });
});
