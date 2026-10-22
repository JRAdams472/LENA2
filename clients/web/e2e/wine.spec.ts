import { test, expect } from "@playwright/test";
import { randomInt } from "node:crypto";
import { graphql, mintToken, unique, uniqueCode } from "./helpers";

test.describe("wine", () => {
  test("create a bottle, find it by vineyard search, delete it", async ({
    page,
    request,
  }) => {
    const token = await mintToken(request);

    // Bottles require existing type, country, and region ids.
    const typeName = unique("E2E Type");
    const countryName = unique("E2E Country");
    const regionName = unique("E2E Region");
    const [type, country] = await Promise.all([
      graphql<{ createType: { id: string } }>(
        request,
        token,
        `mutation ($input: CreateTypeInput!) {
          createType(input: $input) { id }
        }`,
        { input: { name: typeName, description: null } }
      ),
      graphql<{ createCountry: { id: string } }>(
        request,
        token,
        `mutation ($input: CreateCountryInput!) {
          createCountry(input: $input) { id }
        }`,
        { input: { name: countryName, isoCode: uniqueCode() } }
      ),
    ]);
    const region = await graphql<{ createRegion: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateRegionInput!) {
        createRegion(input: $input) { id }
      }`,
      {
        input: {
          countryId: country.createCountry.id,
          name: regionName,
          description: null,
        },
      }
    );
    // A far-future year avoids colliding with seeded vintages.
    const vintageYear = 2050 + randomInt(50);
    const vintage = await graphql<{ createVintage: { id: string } }>(
      request,
      token,
      `mutation ($input: CreateVintageInput!) {
        createVintage(input: $input) { id }
      }`,
      { input: { year: vintageYear, description: null } }
    );

    const vineyard = unique("E2E Vineyard");
    let bottleId: string | null = null;

    try {
      await page.goto("/member/wine/bottles");
      await page
        .getByRole("button", { name: "Create", exact: true })
        .click();
      const dialog = page.getByRole("dialog");
      // Type/country/region are lookup pickers — open each combobox and
      // choose the option by its display name (options render in a
      // portal, so they resolve against page, not dialog).
      await dialog.getByRole("combobox", { name: "Type" }).click();
      await page.getByRole("option", { name: typeName }).click();
      await dialog.getByRole("combobox", { name: "Country" }).click();
      await page.getByRole("option", { name: countryName }).click();
      await dialog.getByRole("combobox", { name: "Region" }).click();
      await page.getByRole("option", { name: regionName }).click();
      await dialog.getByRole("combobox", { name: "Vintage" }).click();
      await page
        .getByRole("option", { name: String(vintageYear), exact: true })
        .click();
      await dialog.getByLabel("Vineyard").fill(vineyard);
      await dialog.getByLabel("Bottle Size").fill("750ml");
      await dialog.getByRole("button", { name: "Save" }).click();
      await expect(dialog).not.toBeVisible();

      // Search narrows to the new bottle.
      await page.getByLabel("Search").fill(vineyard);
      await page.getByRole("button", { name: "Search" }).click();
      const row = page.getByRole("row", { name: new RegExp(vineyard) });
      await expect(row).toBeVisible();

      // Capture the id for API cleanup, then delete via the UI.
      const listed = await graphql<{
        bottles: { items: { id: string }[] };
      }>(
        request,
        token,
        `query { bottles(page: 1, pageSize: 50) { items { id vineyard } } }`
      );
      bottleId =
        listed.bottles.items.find(
          (b) => (b as { vineyard?: string }).vineyard === vineyard
        )?.id ?? null;

      page.once("dialog", (d) => d.accept());
      await row.locator("button").nth(1).click();
      await expect(page.getByText(vineyard)).toHaveCount(0);
    } finally {
      if (bottleId) {
        await graphql(
          request,
          token,
          `mutation ($id: ID!) { deleteBottle(id: $id) }`,
          { id: bottleId }
        ).catch(() => undefined);
      }
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteRegion(id: $id) }`,
        { id: region.createRegion.id }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteVintage(id: $id) }`,
        { id: vintage.createVintage.id }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteCountry(id: $id) }`,
        { id: country.createCountry.id }
      ).catch(() => undefined);
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteType(id: $id) }`,
        { id: type.createType.id }
      ).catch(() => undefined);
    }
  });
});
