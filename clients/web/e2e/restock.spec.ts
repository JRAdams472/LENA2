import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

/** Monday of the current week as YYYY-MM-DD. */
function thisMonday(): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - ((d.getUTCDay() + 6) % 7));
  return d.toISOString().slice(0, 10);
}

test.describe("restock suggestions", () => {
  test("low pantry item with engagement is suggested and adds to the list", async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const token = await mintToken(request);
    const suggestedName = unique("E2E Restock Flour");
    const excludedName = unique("E2E Restock Sugar");
    const quietName = unique("E2E Restock Salt");

    const cat = await graphql<{ createCategory: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateCategoryInput!) {
        createCategory(input: $input) { id }
      }`,
      { input: { name: unique("E2E RestockCat"), description: null } }
    );

    const createItem = async (name: string) => {
      const res = await graphql<{ createItem: { id: string } }>(
        request,
        token,
        `mutation ($input: CreateItemInput!) {
          createItem(input: $input) { id }
        }`,
        {
          input: { name, categoryId: cat.createCategory.id, unit: "ea" },
        }
      );
      return res.createItem.id;
    };

    const suggestedId = await createItem(suggestedName);
    const excludedId = await createItem(excludedName);
    const quietId = await createItem(quietName);

    // All three pantry holdings sit below a 2-unit minimum.
    for (const itemId of [suggestedId, excludedId, quietId]) {
      await graphql(
        request,
        token,
        `mutation ($itemId: ID!, $minQty: Float) {
          adjustUserItem(itemId: $itemId, quantity: 0, minQty: $minQty) { id }
        }`,
        { itemId, minQty: 2 }
      );
    }

    // Only the first two get household engagement; the quiet one must be
    // filtered out by the engagement requirement.
    for (const itemId of [suggestedId, excludedId]) {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { recordSelection(entityType: item, entityId: $id) }`,
        { id: itemId }
      );
    }

    // A grocery list for this week — the excluded item goes on it directly,
    // so it must not appear as a suggestion.
    const plan = await graphql<{ createMealPlan: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateMealPlanInput!) {
        createMealPlan(input: $input) { id }
      }`,
      { input: { name: unique("E2E RestockPlan"), weekStartDate: thisMonday() } }
    );
    const list = await graphql<{ generateGroceryList: { id: string } }>(
      request,
      token,
      `mutation ($id: ID!) {
        generateGroceryList(mealPlanId: $id) { id }
      }`,
      { id: plan.createMealPlan.id }
    );
    await graphql(
      request,
      token,
      `mutation ($input: AddGroceryItemInput!) {
        addGroceryItem(input: $input) { id }
      }`,
      {
        input: {
          groceryListId: list.generateGroceryList.id,
          itemId: excludedId,
          quantity: 1,
          unit: "ea",
        },
      }
    );

    try {
      // The GraphQL contract: suggested = low + engaged + not on the list.
      const gql = await graphql<{
        suggestedRestockItems: { id: string; name: string }[];
      }>(
        request,
        token,
        `query { suggestedRestockItems(limit: 25) { id name } }`
      );
      const ids = gql.suggestedRestockItems.map((i) => i.id);
      expect(ids).toContain(suggestedId);
      expect(ids).not.toContain(excludedId);
      expect(ids).not.toContain(quietId);

      // The list page renders the suggestion and can add it in one click.
      await page.goto(`/grocery-lists/${list.generateGroceryList.id}`);
      await expect(page.getByText("Time to restock")).toBeVisible();
      const row = page
        .locator("div")
        .filter({ hasText: new RegExp(`^${suggestedName}`) })
        .last();
      await expect(row).toBeVisible();
      await row.getByRole("button", { name: "Add" }).click();

      // After adding, the item lands in the unassigned route group.
      const unassignedSection = page
        .locator("div.MuiPaper-root")
        .filter({ has: page.getByRole("heading", { name: "Other items" }) });
      await expect(
        unassignedSection.getByText(suggestedName)
      ).toBeVisible({ timeout: 20_000 });
    } finally {
      for (const itemId of [suggestedId, excludedId, quietId]) {
        await graphql(
          request,
          token,
          `mutation ($id: ID!) { deleteItem(id: $id) }`,
          { id: itemId }
        ).catch(() => undefined);
      }
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
});
