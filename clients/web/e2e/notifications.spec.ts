import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

/** Monday of the current week as YYYY-MM-DD. */
function thisMonday(): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - ((d.getUTCDay() + 6) % 7));
  return d.toISOString().slice(0, 10);
}

// Both specs share e2e-user-1's notification prefs — the toggle test can
// suppress the sweep test's reminder when they run in parallel. Serialize.
test.describe.configure({ mode: "serial" });

test.describe("notification manager", () => {
  test("expiring item triggers a reminder that can add a grocery replacement", async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const token = await mintToken(request);
    const itemName = unique("E2E Milk");

    // Seed: catalog item -> pantry holding expiring within the 3-day
    // window -> a meal plan + grocery list for the replacement action.
    const cat = await graphql<{ createCategory: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateCategoryInput!) {
        createCategory(input: $input) { id }
      }`,
      { input: { name: unique("E2E NotifyCat"), description: null } }
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
    const expiresAt = new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString();
    await graphql(
      request,
      token,
      `mutation ($itemId: ID!, $expiresAt: Time) {
        adjustUserItem(itemId: $itemId, quantity: 1, expiresAt: $expiresAt) { id }
      }`,
      { itemId: item.createItem.id, expiresAt }
    );
    const plan = await graphql<{ createMealPlan: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateMealPlanInput!) {
        createMealPlan(input: $input) { id }
      }`,
      { input: { name: unique("E2E NotifyPlan"), weekStartDate: thisMonday() } }
    );
    await graphql(
      request,
      token,
      `mutation ($id: ID!) {
        generateGroceryList(mealPlanId: $id) { id }
      }`,
      { id: plan.createMealPlan.id }
    );

    try {
      // Admin-only deterministic sweep — materializes the reminder now
      // rather than waiting on the hourly scheduler.
      const sweep = await graphql<{ triggerNotificationSweep: number }>(
        request,
        token,
        `mutation { triggerNotificationSweep }`
      );
      expect(sweep.triggerNotificationSweep).toBeGreaterThanOrEqual(1);

      // The bell shows the reminder with its server-rendered text.
      await page.goto("/");
      await page.getByLabel("notifications").click();
      await expect(
        page.getByText(`${itemName} expires soon`)
      ).toBeVisible();

      // The expiry action adds the item to the current grocery list and
      // navigates there. Scope to this item's menuitem — other expiring
      // items may render their own "Add to list" buttons.
      await page
        .getByRole("menuitem", { name: `${itemName} expires` })
        .getByRole("button", { name: "Add to list" })
        .click();
      await expect(page).toHaveURL(/\/grocery-lists/);
      // Row order is unsafe under parallel specs (another test may have
      // created a newer list) — find the list that actually holds the item.
      const lists = await graphql<{
        groceryLists: { items: { id: string }[] };
      }>(
        request,
        token,
        `query { groceryLists(page: 1, pageSize: 10) { items { id } } }`
      );
      let targetList: string | null = null;
      for (const l of lists.groceryLists.items) {
        const detail = await graphql<{
          groceryList: {
            items: {
              item: { name: string } | null;
              manualItemName: string | null;
            }[];
          };
        }>(
          request,
          token,
          `query ($id: ID!) {
            groceryList(id: $id) { items { item { name } manualItemName } }
          }`,
          { id: l.id }
        );
        if (
          detail.groceryList.items.some(
            (i) => (i.item?.name ?? i.manualItemName) === itemName
          )
        ) {
          targetList = l.id;
          break;
        }
      }
      expect(targetList).not.toBeNull();
      await page.goto(`/grocery-lists/${targetList}`);
      await expect(page.getByText(itemName)).toBeVisible();
    } finally {
      // Disable the expiry category so reruns don't pile up reminders
      // for other tests' leftover items; restore it after.
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
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteMealPlan(id: $id) }`,
        { id: plan.createMealPlan.id }
      ).catch(() => undefined);
    }
  });

  test("per-category preference toggles and persists", async ({
    page,
    request,
  }) => {
    const token = await mintToken(request);
    try {
      await page.goto("/notifications");
      await expect(
        page.getByText("Expiring pantry items")
      ).toBeVisible();
      await expect(page.getByText("All notifications")).toBeVisible();

      const expirySwitch = page.getByRole("switch", {
        name: "Enable Expiring pantry items",
      });
      await expect(expirySwitch).toBeChecked();
      await expirySwitch.click();
      await expect(expirySwitch).not.toBeChecked();

      // Reload — the preference is server-persisted.
      await page.reload();
      await expect(
        page.getByRole("switch", { name: "Enable Expiring pantry items" })
      ).not.toBeChecked();
    } finally {
      // Restore the default so other specs/reruns see the category on.
      await graphql(
        request,
        token,
        `mutation {
          setNotificationCategoryEnabled(category: "expiry", enabled: true)
        }`
      ).catch(() => undefined);
    }
  });
});
