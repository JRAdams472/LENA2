import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

/** Monday of the current week as YYYY-MM-DD. */
function thisMonday(): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - ((d.getUTCDay() + 6) % 7));
  return d.toISOString().slice(0, 10);
}

test.describe("grocery store routing", () => {
  test("store picker, aisle grouping, and cross-aisle move", async ({
    page,
    request,
  }) => {
    test.setTimeout(90_000);
    const token = await mintToken(request);
    const storeName = unique("E2E Market");
    const apples = unique("E2E Apples");
    const milk = unique("E2E Milk");
    const bread = unique("E2E Bread");

    // A store with two aisles.
    const store = await graphql<{ createStore: { id: string } }>(
      request,
      token,
      `mutation ($name: String!) { createStore(name: $name) { id } }`,
      { name: storeName }
    );
    const storeId = store.createStore.id;
    const aisleIds: Record<string, string> = {};
    for (const [i, aisle] of ["Produce", "Dairy"].entries()) {
      const res = await graphql<{ createStoreAisle: { id: string } }>(
        request,
        token,
        `mutation ($storeId: ID!, $name: String!, $position: Int!) {
          createStoreAisle(storeId: $storeId, name: $name, position: $position) { id }
        }`,
        { storeId, name: aisle, position: i }
      );
      aisleIds[aisle] = res.createStoreAisle.id;
    }

    // A list bound to the store, holding three manual items.
    const plan = await graphql<{ createMealPlan: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateMealPlanInput!) {
        createMealPlan(input: $input) { id }
      }`,
      { input: { name: unique("E2E RoutePlan"), weekStartDate: thisMonday() } }
    );
    const list = await graphql<{ generateGroceryList: { id: string } }>(
      request,
      token,
      `mutation ($id: ID!) { generateGroceryList(mealPlanId: $id) { id } }`,
      { id: plan.createMealPlan.id }
    );
    const listId = list.generateGroceryList.id;
    await graphql(
      request,
      token,
      `mutation ($groceryListId: ID!, $storeId: ID) {
        setGroceryListStore(groceryListId: $groceryListId, storeId: $storeId) { id }
      }`,
      { groceryListId: listId, storeId }
    );
    for (const name of [apples, milk, bread]) {
      await graphql(
        request,
        token,
        `mutation ($input: AddGroceryItemInput!) {
          addGroceryItem(input: $input) { id }
        }`,
        {
          input: {
            groceryListId: listId,
            manualItemName: name,
            quantity: 1,
            unit: "ea",
          },
        }
      );
    }

    // Apples → Produce, Milk → Dairy; Bread stays unassigned.
    const assign = (aisleId: string, manualItemName: string) =>
      graphql(
        request,
        token,
        `mutation ($storeId: ID!, $aisleId: ID, $manualItemName: String) {
          assignItemToAisle(storeId: $storeId, aisleId: $aisleId, manualItemName: $manualItemName)
        }`,
        { storeId, aisleId, manualItemName }
      );
    await assign(aisleIds["Produce"], apples);
    await assign(aisleIds["Dairy"], milk);

    const milkRowId = (await itemIdsOf(request, token, listId, [milk]))[0];

    const section = (heading: string) =>
      page
        .locator("div.MuiPaper-root")
        .filter({ has: page.getByRole("heading", { name: heading }) });

    try {
      await page.goto(`/grocery-lists/${listId}`);

      // The picker shows the persisted store and the list renders in
      // server-computed route groups.
      await expect(page.getByText(storeName)).toBeVisible();
      await expect(
        section("Produce").getByText(apples)
      ).toBeVisible();
      await expect(section("Dairy").getByText(milk)).toBeVisible();
      await expect(
        section("Other items").getByText(bread)
      ).toBeVisible();

      // Move Milk to Produce through the row's move-to menu — the cross-
      // aisle write goes through reorderGroceryListItems server-side.
      await page
        .getByTestId(`grocery-row-${milkRowId}`)
        .getByRole("button", { name: "item actions" })
        .click();
      await page
        .getByRole("menuitem", { name: "Move to Produce" })
        .click();
      await expect(
        section("Produce").getByText(milk)
      ).toBeVisible({ timeout: 20_000 });
      await expect(section("Dairy").getByText(milk)).toHaveCount(0);

      // A server-side reorder is reflected verbatim — the UI never sorts.
      // Entries are the flat display order; null aisleId means "no change",
      // so Milk and Apples stay in Produce but swap positions.
      const ids = await itemIdsOf(request, token, listId, [milk, apples, bread]);
      await graphql(
        request,
        token,
        `mutation ($groceryListId: ID!, $entries: [GroceryReorderEntryInput!]!) {
          reorderGroceryListItems(groceryListId: $groceryListId, entries: $entries)
        }`,
        {
          groceryListId: listId,
          entries: ids.map((id) => ({ groceryListItemId: id })),
        }
      );
      await page.reload();
      const produceText = await section("Produce").innerText();
      expect(produceText.indexOf(milk)).toBeLessThan(
        produceText.indexOf(apples)
      );
    } finally {
      await graphql(
        request,
        token,
        `mutation ($storeId: ID!) { deleteStore(storeId: $storeId) }`,
        { storeId }
      ).catch(() => undefined);
      // grocery_list has an FK to meal_plan — tolerate the violation.
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteMealPlan(id: $id) }`,
        { id: plan.createMealPlan.id }
      ).catch(() => undefined);
    }
  });
});

/** Looks up grocery-list item row ids by display name, in the given order. */
async function itemIdsOf(
  request: Parameters<typeof graphql>[0],
  token: string,
  listId: string,
  names: string[]
): Promise<string[]> {
  const detail = await graphql<{
    groceryList: {
      items: { id: string; manualItemName: string | null }[];
    };
  }>(
    request,
    token,
    `query ($id: ID!) { groceryList(id: $id) { items { id manualItemName } } }`,
    { id: listId }
  );
  return names.map((name) => {
    const row = detail.groceryList.items.find(
      (i) => i.manualItemName === name
    );
    if (!row) throw new Error(`no grocery item named ${name}`);
    return row.id;
  });
}
