import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

// The web pantry lives at /inventory/items (there is no /pantry route).
// The expiry reminder's "Add to list" action is already covered by
// notifications.spec.ts, so this spec stays on the holding fields.
test.describe("pantry quantities", () => {
  test("set quantity and minimum from the inventory page", async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const token = await mintToken(request);
    const name = unique("E2E Pantry Beans");

    const cat = await graphql<{ createCategory: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateCategoryInput!) {
        createCategory(input: $input) { id }
      }`,
      { input: { name: unique("E2E PantryCat"), description: null } }
    );
    const item = await graphql<{ createItem: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateItemInput!) {
        createItem(input: $input) { id }
      }`,
      { input: { name, categoryId: cat.createCategory.id, unit: "ea" } }
    );
    const itemId = item.createItem.id;
    const holding = async () => {
      const res = await graphql<{
        userItems: {
          items: { currentQty: number; minQty: number | null; expiresAt: string | null }[];
        };
      }>(
        request,
        token,
        `query ($ids: [ID!]) {
          userItems(itemIds: $ids) { items { currentQty minQty expiresAt } }
        }`,
        { ids: [itemId] }
      );
      return res.userItems.items[0] ?? null;
    };

    try {
      await page.goto("/inventory/items");
      await page.getByLabel("Search").fill(name);
      const row = page.getByRole("row", { name: new RegExp(name) });
      await expect(row).toBeVisible();

      // Quantity, minimum, and expiry through the edit dialog.
      await row.getByRole("button", { name: "Edit" }).click();
      const dialog = page.getByRole("dialog");
      await dialog.getByLabel("Current Quantity").fill("3");
      await dialog.getByLabel("Min Quantity").fill("2");
      await dialog.getByRole("button", { name: "Save" }).click();
      await expect(dialog).toBeHidden();

      await expect(row.getByTitle("Adjust quantity")).toHaveText(/^3 /);
      await expect
        .poll(holding)
        .toMatchObject({ currentQty: 3, minQty: 2 });

      // The quantity chip adjusts in place and drops below the minimum.
      await row.getByTitle("Adjust quantity").click();
      const qty = page.getByRole("dialog", { name: `Edit ${name} Quantity` });
      await qty.getByRole("textbox", { name: "Quantity" }).fill("1");
      await qty.getByRole("button", { name: "Save" }).click();
      await expect(qty).toBeHidden();
      await expect(row.getByTitle("Adjust quantity")).toHaveText(/^1 /);
      await expect.poll(async () => (await holding())?.currentQty).toBe(1);

      // State persists across a reload.
      await page.reload();
      await page.getByLabel("Search").fill(name);
      const reloaded = page.getByRole("row", { name: new RegExp(name) });
      await expect(reloaded.getByTitle("Adjust quantity")).toHaveText(/^1 /);
      await expect(reloaded).toContainText("2");
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteItem(id: $id) }`,
        { id: itemId }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteCategory(id: $id) }`,
        { id: cat.createCategory.id }
      ).catch(() => undefined);
    }
  });

  // FINDING: the edit dialog sends Expiry Date as YYYY-MM-DD to
  // adjustUserItem(expiresAt: Time), which only accepts RFC 3339; the call
  // is fire-and-forget, so the error is swallowed and quantity, minimum,
  // and expiry are all dropped. Remove test.fail() once fixed.
  test("an expiry date set in the edit dialog persists", async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    test.fail();
    const token = await mintToken(request);
    const expiry = new Date(Date.now() + 3 * 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);
    const name = unique("E2E Pantry Expiry");

    const cat = await graphql<{ createCategory: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateCategoryInput!) {
        createCategory(input: $input) { id }
      }`,
      { input: { name: unique("E2E PantryCat"), description: null } }
    );
    const item = await graphql<{ createItem: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateItemInput!) {
        createItem(input: $input) { id }
      }`,
      { input: { name, categoryId: cat.createCategory.id, unit: "ea" } }
    );
    const itemId = item.createItem.id;
    const holding = async () => {
      const res = await graphql<{
        userItems: {
          items: { currentQty: number; minQty: number | null; expiresAt: string | null }[];
        };
      }>(
        request,
        token,
        `query ($ids: [ID!]) {
          userItems(itemIds: $ids) { items { currentQty minQty expiresAt } }
        }`,
        { ids: [itemId] }
      );
      return res.userItems.items[0] ?? null;
    };

    try {
      await page.goto("/inventory/items");
      await page.getByLabel("Search").fill(name);
      const row = page.getByRole("row", { name: new RegExp(name) });
      await expect(row).toBeVisible();

      await row.getByRole("button", { name: "Edit" }).click();
      const dialog = page.getByRole("dialog");
      await dialog.getByLabel("Current Quantity").fill("3");
      await dialog.getByLabel("Expiry Date").fill(expiry);
      await dialog.getByRole("button", { name: "Save" }).click();
      await expect(dialog).toBeHidden();
      await expect
        .poll(async () => (await holding())?.expiresAt?.slice(0, 10) ?? null)
        .toBe(expiry);
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteItem(id: $id) }`,
        { id: itemId }
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
