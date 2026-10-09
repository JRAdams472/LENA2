import { test, expect, type Page } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

/** Monday of the current week as YYYY-MM-DD. */
function thisMonday(): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - ((d.getUTCDay() + 6) % 7));
  return d.toISOString().slice(0, 10);
}

/** Row order of the list page, by the names it renders. */
async function rowIndex(page: Page, names: string[]): Promise<number[]> {
  const rows = await page.getByTestId(/^grocery-row-/).allTextContents();
  return names.map((n) => rows.findIndex((r) => r.includes(n)));
}

test.describe("grocery check-off", () => {
  test("generated list check-off persists, credits the pantry, and manual order survives regenerate", async ({
    page,
    request,
  }) => {
    test.setTimeout(90_000);
    const token = await mintToken(request);
    const itemName = unique("E2E Checkoff Rice");
    const manualName = unique("E2E Checkoff Candles");

    const cat = await graphql<{ createCategory: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateCategoryInput!) {
        createCategory(input: $input) { id }
      }`,
      { input: { name: unique("E2E CheckoffCat"), description: null } }
    );
    const item = await graphql<{ createItem: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateItemInput!) {
        createItem(input: $input) { id }
      }`,
      { input: { name: itemName, categoryId: cat.createCategory.id, unit: "ea" } }
    );
    const itemId = item.createItem.id;
    // Empty pantry holding, so the whole recipe need lands on the list.
    await graphql(
      request,
      token,
      `mutation ($itemId: ID!) {
        adjustUserItem(itemId: $itemId, quantity: 0) { id }
      }`,
      { itemId }
    );
    const recipe = await graphql<{ createRecipe: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateRecipeInput!) {
        createRecipe(input: $input) { id }
      }`,
      {
        input: {
          name: unique("E2E Checkoff Recipe"),
          servings: 2,
          items: [{ itemId, quantity: 2, unit: "ea" }],
          steps: [{ stepNumber: 1, instruction: "Cook the rice" }],
        },
      }
    );
    const plan = await graphql<{ createMealPlan: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateMealPlanInput!) {
        createMealPlan(input: $input) { id }
      }`,
      { input: { name: unique("E2E CheckoffPlan"), weekStartDate: thisMonday() } }
    );
    const planId = plan.createMealPlan.id;

    const pantryQty = async (): Promise<number> => {
      const res = await graphql<{
        userItems: { items: { currentQty: number }[] };
      }>(
        request,
        token,
        `query ($ids: [ID!]) {
          userItems(itemIds: $ids) { items { currentQty } }
        }`,
        { ids: [itemId] }
      );
      return res.userItems.items[0]?.currentQty ?? 0;
    };
    const generate = async (): Promise<string> => {
      const res = await graphql<{ generateGroceryList: { id: string } }>(
        request,
        token,
        `mutation ($id: ID!) { generateGroceryList(mealPlanId: $id) { id } }`,
        { id: planId }
      );
      return res.generateGroceryList.id;
    };

    try {
      await graphql(
        request,
        token,
        `mutation ($input: AddMealSlotInput!) { addMealSlot(input: $input) { id } }`,
        {
          input: {
            mealPlanId: planId,
            dayOfWeek: 0,
            mealType: "Dinner",
            recipeId: recipe.createRecipe.id,
            servings: 2,
          },
        }
      );
      const listId = await generate();
      const manual = await graphql<{ addGroceryItem: { id: string } }>(
        request,
        token,
        `mutation ($input: AddGroceryItemInput!) { addGroceryItem(input: $input) { id } }`,
        {
          input: {
            groceryListId: listId,
            manualItemName: manualName,
            quantity: 1,
            unit: "ea",
            source: "manual",
          },
        }
      );
      const lines = await graphql<{
        groceryList: { items: { id: string; item: { id: string } | null }[] };
      }>(
        request,
        token,
        `query ($id: ID!) { groceryList(id: $id) { items { id item { id } } } }`,
        { id: listId }
      );
      const generatedLine = lines.groceryList.items.find((l) => l.item?.id === itemId);
      expect(generatedLine).toBeDefined();

      // Manual order (what the drag handle submits): the manual line first.
      await graphql(
        request,
        token,
        `mutation ($id: ID!, $entries: [GroceryReorderEntryInput!]!) {
          reorderGroceryListItems(groceryListId: $id, entries: $entries)
        }`,
        {
          id: listId,
          entries: [
            { groceryListItemId: manual.addGroceryItem.id },
            { groceryListItemId: generatedLine!.id },
          ],
        }
      );

      await page.goto(`/grocery-lists/${listId}`);
      await expect(page.getByText(itemName)).toBeVisible();
      let [m, g] = await rowIndex(page, [manualName, itemName]);
      expect(m).toBeGreaterThanOrEqual(0);
      expect(m).toBeLessThan(g);

      // Regenerating in place replaces the generated lines; the manual
      // line and the household's manual ranking both survive.
      expect(await generate()).toBe(listId);
      await page.reload();
      await expect(page.getByText(itemName)).toBeVisible();
      [m, g] = await rowIndex(page, [manualName, itemName]);
      expect(m).toBeGreaterThanOrEqual(0);
      expect(m).toBeLessThan(g);

      // Check the generated line off in the UI.
      expect(await pantryQty()).toBe(0);
      const box = page.getByRole("checkbox", { name: new RegExp(itemName) });
      // Controlled checkbox: the state flips once the mutation lands.
      await box.click();
      await expect(box).toBeChecked();
      await expect.poll(pantryQty).toBe(2);

      // Check state is server-persisted.
      await page.reload();
      await expect(
        page.getByRole("checkbox", { name: new RegExp(itemName) })
      ).toBeChecked();

      // Unchecking reverses the pantry credit.
      await page.getByRole("checkbox", { name: new RegExp(itemName) }).click();
      await expect.poll(pantryQty).toBe(0);
    } finally {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteMealPlan(id: $id) }`,
        { id: planId }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
        { id: recipe.createRecipe.id }
      ).catch(() => undefined);
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
