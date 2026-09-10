import { test, expect, APIRequestContext } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

test.setTimeout(120000);

interface ItemNode {
  id: string;
  name: string;
}

async function findItemByName(
  request: APIRequestContext,
  token: string,
  itemName: string
): Promise<ItemNode | undefined> {
  let page = 1;
  for (;;) {
    const data = await graphql<{
      items: { items: ItemNode[]; pageInfo: { totalCount: number } };
    }>(
      request,
      token,
      `query ($page: Int!, $pageSize: Int!) {
        items(page: $page, pageSize: $pageSize) {
          items { id name }
          pageInfo { totalCount }
        }
      }`,
      { page, pageSize: 100 }
    );
    const found = data.items.items.find((it) => it.name === itemName);
    if (found) return found;
    const seen = page * 100;
    if (seen >= data.items.pageInfo.totalCount || data.items.items.length === 0) break;
    page++;
  }
  return undefined;
}

test.describe("items", () => {
  test("create, search, favorite, and delete an item", async ({
    request,
  }) => {
    const token = await mintToken(request);

    const catName = unique("E2E ItemCat");
    const cat = await graphql<{ createCategory: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateCategoryInput!) {
        createCategory(input: $input) { id }
      }`,
      { input: { name: catName, description: null } }
    );
    const categoryId = cat.createCategory.id;
    const itemName = unique("E2E Item");

    try {
      // Create the item.
      await graphql<{ createItem: { id: string } }>(
        request,
        token,
        `mutation ($input: CreateItemInput!) {
          createItem(input: $input) { id }
        }`,
        {
          input: {
            name: itemName,
            brandId: null,
            upc12: null,
            upc14: null,
            categoryId,
            unit: "ea",
          },
        }
      );

      // Search for it by name.
      const item = await findItemByName(request, token, itemName);
      expect(item).toBeTruthy();

      // Toggle favorite.
      await graphql(
        request,
        token,
        `mutation ($itemId: ID!, $isFavorite: Boolean!) {
          setItemFavorite(itemId: $itemId, isFavorite: $isFavorite) { id }
        }`,
        { itemId: item!.id, isFavorite: true }
      );

      // Verify it remains findable.
      const favItem = await findItemByName(request, token, itemName);
      expect(favItem).toBeTruthy();

      // Delete it.
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteItem(id: $id) }`,
        { id: item!.id }
      );
    } finally {
      const item = await findItemByName(request, token, itemName);
      if (item) {
        await graphql(
          request,
          token,
          `mutation ($id: ID!) { deleteItem(id: $id) }`,
          { id: item.id }
        );
      }
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteCategory(id: $id) }`,
        { id: categoryId }
      );
    }
  });
});
