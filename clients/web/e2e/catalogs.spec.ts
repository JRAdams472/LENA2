import { test, expect, Page } from "@playwright/test";
import { graphql, mintToken, unique, uniqueCode } from "./helpers";

interface FieldFill {
  label: string;
  value: string;
}

interface Catalog {
  name: string;
  url: string;
  fields: FieldFill[];
  /** Text expected to appear in the created row. */
  rowText: string;
  /** Large paged catalogs require a search to surface the created row. */
  search?: boolean;
}

const catalogs: Catalog[] = [
  {
    name: "Brands",
    url: "/inventory/brands",
    fields: [{ label: "Brand Name", value: unique("E2E Brand") }],
    rowText: "",
    search: true,
  },
  {
    name: "Categories",
    url: "/inventory/categories",
    fields: [
      { label: "Name", value: unique("E2E Category") },
      { label: "Description", value: "created by e2e" },
    ],
    rowText: "",
  },
  {
    name: "Flavor Profiles",
    url: "/inventory/flavor-profiles",
    fields: [{ label: "Flavor Name", value: unique("E2E Flavor") }],
    rowText: "",
  },
  {
    name: "Nutrient Types",
    url: "/inventory/nutrient-types",
    fields: [
      { label: "Nutrient Name", value: unique("E2E Nutrient") },
      { label: "Unit of Measure", value: "mg" },
    ],
    rowText: "",
  },
  {
    name: "Countries",
    url: "/wine/countries",
    fields: [
      { label: "Country Name", value: unique("E2E Country") },
      { label: "Country Code", value: uniqueCode() },
    ],
    rowText: "",
  },
  {
    name: "Types",
    url: "/wine/types",
    fields: [{ label: "Type Name", value: unique("E2E Type") }],
    rowText: "",
  },
  {
    name: "Vintages",
    url: "/wine/vintages",
    fields: [{ label: "Year", value: "2001" }],
    rowText: "2001",
  },
  {
    name: "Grape Varieties",
    url: "/wine/grape-varieties",
    fields: [{ label: "Name", value: unique("E2E Grape") }],
    rowText: "",
  },
  {
    name: "Wine Flavor Profiles",
    url: "/wine/wine-flavor-profiles",
    fields: [{ label: "Name", value: unique("E2E Wine Flavor") }],
    rowText: "",
  },
];

async function createViaDialog(page: Page, fields: FieldFill[]) {
  await page.getByRole("button", { name: "Create", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  for (const f of fields) {
    await dialog.getByLabel(f.label).fill(f.value);
  }
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog).not.toBeVisible();
}

async function deleteRow(page: Page, text: string) {
  page.once("dialog", (d) => d.accept());
  const row = page.getByRole("row").filter({ hasText: text }).first();
  await row.getByRole("button", { name: "Delete" }).click();
}

// Refetch-dependent assertions: a row appears/disappears only once the
// mutation resolves and the list refetches — large catalogs (Brands walks
// ~91 pages then re-renders ~9k rows) make the default 5s poll racy.
const REFETCH_TIMEOUT = { timeout: 20000 };

async function expectRowVisible(page: Page, text: string) {
  await expect(page.getByText(text).first()).toBeVisible(REFETCH_TIMEOUT);
}

async function expectRowGone(page: Page, text: string) {
  await expect(page.getByText(text)).toHaveCount(0, REFETCH_TIMEOUT);
}

for (const catalog of catalogs) {
  test(`catalog CRUD: ${catalog.name}`, async ({ page }) => {
    const expected = catalog.rowText || catalog.fields[0].value;

    await page.goto(catalog.url);
    await expect(
      page.getByRole("heading", { name: catalog.name })
    ).toBeVisible();

    await createViaDialog(page, catalog.fields);
    // Server-paged catalogs sort the new row off page 1 — search it.
    if (catalog.search) await page.getByLabel("Search").fill(expected);
    await expectRowVisible(page, expected);

    await deleteRow(page, expected);
    await expectRowGone(page, expected);
  });
}

test("catalog CRUD: Regions (needs a country)", async ({
  page,
  request,
}) => {
  const token = await mintToken(request);
  const countryName = unique("E2E RegionHost");
  const country = await graphql<{ createCountry: { id: string } }>(
    request,
    token,
    `mutation ($input: CreateCountryInput!) {
      createCountry(input: $input) { id }
    }`,
    { input: { name: countryName, isoCode: uniqueCode() } }
  );
  const countryId = country.createCountry.id;
  const regionName = unique("E2E Region");

  try {
    await page.goto("/wine/regions");
    await createViaDialog(page, [
      { label: "Region Name", value: regionName },
      { label: "Country ID", value: countryId },
    ]);
    await expectRowVisible(page, regionName);

    await deleteRow(page, regionName);
    await expectRowGone(page, regionName);
  } finally {
    // Clean up the region before deleting its country.
    const regions = await graphql<{
      regions: { id: string; name: string }[];
    }>(
      request,
      token,
      `query ($countryId: ID!) {
        regions(countryId: $countryId) { id name }
      }`,
      { countryId }
    );
    const region = regions.regions.find((r) => r.name === regionName);
    if (region) {
      await graphql(
        request,
        token,
        `mutation ($id: ID!) { deleteRegion(id: $id) }`,
        { id: region.id }
      );
    }
    await graphql(
      request,
      token,
      `mutation ($id: ID!) { deleteCountry(id: $id) }`,
      { id: countryId }
    );
  }
});
