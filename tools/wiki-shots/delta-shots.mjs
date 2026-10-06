// Capture recipe-delta screenshots against the lena2shots e2e stack.
// Assumes: stack up, seed_demo.py run, and a delta already saved on
// "Garlic Butter Pasta" (see the setRecipeDelta call in the phase notes).
//
//   node tools/wiki-shots/delta-shots.mjs
//
// Env: SHOTS_OUT, SHOTS_BASE, SHOTS_ISSUER — same as capture.mjs.

import { createRequire } from "node:module";
import { mkdirSync } from "node:fs";

const require = createRequire(new URL("../../clients/web/package.json", import.meta.url));
const { chromium, request } = require("playwright");

const OUT = process.env.SHOTS_OUT ?? "wiki-shots";
const BASE = process.env.SHOTS_BASE ?? "http://localhost";
const ISSUER = process.env.SHOTS_ISSUER ?? "http://localhost:8085";

mkdirSync(OUT, { recursive: true });

const ctx = await request.newContext();
const res = await ctx.get(
  `${ISSUER}/token?sub=e2e-user-1&email=e2e%40example.com&name=E2E%20User`
);
const { id_token } = await res.json();

const browser = await chromium.launch();

function newPage(viewport) {
  const context = browser.newContext({
    viewport,
    deviceScaleFactor: 2,
  });
  return context.then(async (c) => {
    await c.addInitScript(
      (t) => window.localStorage.setItem("lena_id_token", t),
      id_token
    );
    return c.newPage();
  });
}

async function shot(page, name, url, waitText, opts = {}) {
  await page.goto(`${BASE}${url}`, { waitUntil: "domcontentloaded" });
  await page.getByText(waitText).first().waitFor({ timeout: 30000 });
  await page.waitForTimeout(900);
  await page.screenshot({ path: `${OUT}/${name}.png`, fullPage: opts.fullPage ?? true });
  console.log(`shot: ${name}`);
}

const page = await newPage({ width: 1440, height: 900 });

// 1. Effective view — chip, view toggle, per-line badges, tweaks panel.
await shot(page, "recipe-delta-household", "/recipes/2", "Household tweaks");

// 2. Per-line tweak editor open on the salt line.
{
  await page.goto(`${BASE}/recipes/2`, { waitUntil: "domcontentloaded" });
  await page.getByText("Household tweaks").first().waitFor({ timeout: 30000 });
  const tweakButtons = page.getByRole("button", { name: "Tweak" });
  await tweakButtons.first().waitFor({ timeout: 10000 });
  // Open the tweak editor on the first tweakable row.
  await tweakButtons.first().click();
  await page.getByText("Apply tweak").waitFor({ timeout: 10000 });
  await page.waitForTimeout(400);
  await page.screenshot({ path: `${OUT}/recipe-delta-tweak-editor.png` });
  console.log("shot: recipe-delta-tweak-editor");
  await page.getByRole("button", { name: "Cancel" }).click();
}

// 3. Canonical view via the toggle.
{
  await page.getByRole("button", { name: "Original recipe" }).click();
  await page
    .getByText("Showing the original recipe")
    .waitFor({ timeout: 10000 });
  await page.waitForTimeout(800);
  await page.screenshot({ path: `${OUT}/recipe-delta-original.png`, fullPage: true });
  console.log("shot: recipe-delta-original");
  await page.getByRole("button", { name: "Household version" }).click();
  await page.getByText("Showing the original recipe").waitFor({ state: "detached", timeout: 10000 }).catch(() => {});
}

// 4. Stale state — a canonical edit bumps updated_at and orphans the
// removed-line tweak (the canonical write drops that same line).
{
  const r = await ctx.post(`${BASE}/graphql`, {
    headers: { Authorization: `Bearer ${id_token}` },
    data: {
      query: `mutation ($id: ID!, $input: CreateRecipeInput!) {
        updateRecipe(id: $id, input: $input) { id }
      }`,
      variables: {
        id: "2",
        input: {
          name: "Garlic Butter Pasta",
          description: "Pasta tossed in browned garlic butter with shaved cheese.",
          servings: 2,
          prepTimeMinutes: 5,
          cookTimeMinutes: 15,
          categoryIds: null,
          items: [
            { itemId: "40476", ingredientId: null, quantity: 1, unit: "pound", notes: null, isOptional: false },
            { itemId: "27841", ingredientId: null, quantity: 4, unit: "tablespoon", notes: null, isOptional: false },
            { itemId: null, ingredientId: "2", quantity: 3, unit: "each", notes: null, isOptional: false },
            { itemId: null, ingredientId: "3", quantity: 1, unit: "teaspoon", notes: null, isOptional: false },
          ],
          steps: [
            { stepNumber: 1, instruction: "Boil salted water and cook the pasta" },
            { stepNumber: 2, instruction: "Melt butter with garlic until fragrant" },
            { stepNumber: 3, instruction: "Toss pasta in the garlic butter with cheese" },
            { stepNumber: 4, instruction: "Serve immediately" },
          ],
        },
      },
    },
  });
  const body = await r.json();
  if (body.errors) console.log("updateRecipe errors:", JSON.stringify(body.errors));
  await page.goto(`${BASE}/recipes/2`, { waitUntil: "domcontentloaded" });
  await page
    .getByText(/original recipe changed after these tweaks/)
    .waitFor({ timeout: 30000 });
  await page.waitForTimeout(800);
  await page.screenshot({ path: `${OUT}/recipe-delta-stale.png`, fullPage: true });
  console.log("shot: recipe-delta-stale");
}

// 5. Mobile viewport — effective view collapsed.
{
  const mpage = await newPage({ width: 390, height: 844 });
  await shot(mpage, "recipe-delta-mobile", "/recipes/2", "Household tweaks");
  await mpage.close();
}

await browser.close();
console.log("done");
