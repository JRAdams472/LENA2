import { test, expect } from "@playwright/test";

const API_URL = `${process.env.E2E_BASE_URL ?? "http://localhost"}/graphql`;

test.describe("smoke", () => {
  test("authenticated dashboard loads with navigation", async ({ page }) => {
    await page.goto("/");

    await expect(page).toHaveTitle(/LENA/);
    await expect(page.getByText("Inventory").first()).toBeVisible();
    await expect(page.getByText("Wine").first()).toBeVisible();
    await expect(page.getByText("Meal Planning").first()).toBeVisible();
  });

  test("navigation links work", async ({ page }) => {
    await page.goto("/");
    await page.getByText("Inventory").first().click();
    await page.getByRole("link", { name: "Categories" }).first().click();
    await expect(page).toHaveURL(/\/inventory\/categories/);
    await expect(
      page.getByRole("heading", { name: "Categories" })
    ).toBeVisible();
  });

  test("/health returns 200", async ({ request }) => {
    const res = await request.get(
      `${process.env.E2E_BASE_URL ?? "http://localhost"}/health`
    );
    expect(res.status()).toBe(200);
  });

  test("graphql endpoint rejects unauthenticated requests", async ({
    request,
  }) => {
    const res = await request.post(API_URL, {
      data: { query: "{ __typename }" },
    });
    expect(res.status()).toBe(401);
  });
});
