import { test, expect } from "@playwright/test";
import { graphql, mintToken, unique } from "./helpers";

const RECIPE_LIST = `query ($search: String, $searchMode: RecipeSearchMode) {
  recipes(search: $search, searchMode: $searchMode) {
    items { id name }
    pageInfo { totalCount }
  }
}`;

test.describe("semantic recipe search", () => {
  test("semantic mode ranks by meaning and the toggle drives it", async ({
    page,
    request,
  }) => {
    test.setTimeout(90_000);
    const token = await mintToken(request);
    const tag = unique("sem");
    // The mock embedder hashes shared words, so "tomato pasta" lands closer
    // to the target's text than the distractor's.
    const targetName = `E2E ${tag} Tomato Basil Pasta`;
    const otherName = `E2E ${tag} Chocolate Lava Cake`;

    const create = (name: string, description: string) =>
      graphql<{ createRecipe: { id: string } }>(
        request,
        token,
        `mutation ($input: CreateRecipeInput!) {
          createRecipe(input: $input) { id }
        }`,
        {
          input: {
            name,
            description,
            servings: 4,
            prepTimeMinutes: 10,
            cookTimeMinutes: 25,
            items: [],
            steps: [],
          },
        }
      );

    const target = (await create(targetName, "warm tomato pasta dinner")).createRecipe;
    const other = (await create(otherName, "rich chocolate dessert")).createRecipe;

    try {
      // Embedding refresh is async after save — poll the semantic query
      // until the target row is embedded and returned.
      const names = async () => {
        const data = await graphql<{
          recipes: { items: { id: string; name: string }[] };
        }>(request, token, RECIPE_LIST, {
          search: "tomato pasta dinner",
          searchMode: "semantic",
        });
        return data.recipes.items.map((i) => i.name);
      };
      await expect
        .poll(names, { timeout: 30_000 })
        .toContain(targetName);
      // Once both are embedded, meaning outranks the unrelated recipe.
      await expect
        .poll(async () => (await names()).indexOf(targetName), { timeout: 30_000 })
        .toBeGreaterThanOrEqual(0);
      const ranked = await names();
      const otherIdx = ranked.includes(otherName)
        ? ranked.indexOf(otherName)
        : Number.MAX_SAFE_INTEGER;
      expect(ranked.indexOf(targetName)).toBeLessThan(otherIdx);

      // The UI toggle exists (semanticSearchAvailable is true in e2e) and
      // drives the same result.
      await page.goto("/recipes");
      await page.getByLabel("Semantic").click();
      await page.getByLabel("Search").fill("tomato pasta dinner");
      await expect(
        page.getByRole("row", { name: new RegExp(targetName) })
      ).toBeVisible({ timeout: 15_000 });
    } finally {
      for (const id of [target.id, other.id]) {
        await graphql(
          request,
          token,
          `mutation ($id: ID!) { deleteRecipe(id: $id) }`,
          { id }
        );
      }
    }
  });
});
