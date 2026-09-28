// Capture wiki screenshots against the lena2shots e2e stack.
//
//   node tools/wiki-shots/capture.mjs
//
// Env:
//   SHOTS_OUT   output dir        (default: ./wiki-shots)
//   SHOTS_BASE  app base URL      (default: http://localhost — the lena2shots Caddy)
//   SHOTS_ISSUER test-issuer URL  (default: http://localhost:8085)
//
// playwright is resolved from clients/web/node_modules, so run from the
// repo root after `npm ci` in clients/web and `npx playwright install chromium`.
// The shot list below is the canonical set — add/remove entries for new pages,
// and prefer waitFor text that only appears once real data has rendered.

import { createRequire } from "module";
import { mkdirSync } from "fs";

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
const context = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
});
// The app reads lena_id_token from localStorage on load and migrates it to
// sessionStorage — seed it the same way e2e/auth.setup.ts does.
await context.addInitScript(
  (t) => window.localStorage.setItem("lena_id_token", t),
  id_token
);
const page = await context.newPage();

// Recipe IDs are not stable across fresh volumes — resolve them by name.
async function recipeId(name) {
  const r = await ctx.post(`${BASE}/graphql`, {
    headers: { Authorization: `Bearer ${id_token}` },
    data: {
      query: `query ($s: String) { recipes(page: 1, pageSize: 5, search: $s) { items { id name } } }`,
      variables: { s: name },
    },
  });
  const body = await r.json();
  const hit = body.data?.recipes?.items?.find((i) => i.name === name);
  if (!hit) throw new Error(`recipe not found: ${name}`);
  return hit.id;
}

const roastId = await recipeId("Herb Roast Chicken with Vegetables");

async function shot(name, path, waitFor) {
  // NOTE: "networkidle" never settles — the app polls notifications.
  await page.goto(`${BASE}${path}`, { waitUntil: "domcontentloaded" });
  if (waitFor) {
    await page.getByText(waitFor, { exact: false }).first().waitFor({ timeout: 20000 });
  }
  await page.waitForTimeout(1200);
  await page.screenshot({ path: `${OUT}/${name}.png`, fullPage: false });
  console.log("shot:", name);
}

await shot("dashboard", "/", "Garlic Butter Pasta");
await shot("recipes", "/recipes", "Herb Roast Chicken");
await shot("recipe-detail", `/recipes/${roastId}`, "Herb Roast Chicken");
await shot("recipe-categories-admin", "/recipes/categories", "Cuisine");

// Category filter in action — open the Cuisine dropdown on the recipes list.
await page.goto(`${BASE}/recipes`, { waitUntil: "domcontentloaded" });
await page.getByText("Herb Roast Chicken").first().waitFor({ timeout: 20000 });
await page.getByRole("combobox", { name: "Cuisine" }).click();
await page.getByRole("option", { name: "Italian" }).waitFor({ timeout: 10000 });
await page.waitForTimeout(400);
await page.screenshot({ path: `${OUT}/recipe-category-filter.png` });
await page.keyboard.press("Escape");
console.log("shot: recipe-category-filter");
await shot("meal-plans", "/meal-plans", "Week of");
await shot("meal-plan-week", "/meal-plans/1", "Week of");
await shot("grocery-lists", "/grocery-lists", "20");
await shot("grocery-list", "/grocery-lists/1", "Beef Broth");
await shot("events", "/events", "Autumn Dinner Party");
await shot("event-detail", "/events/1", "Autumn Dinner Party");
await shot("wine-bottles", "/wine/bottles", "Silver Oak");
await shot("household", "/household", "E2E Other");
// The items page pages the whole catalog client-side — give it time.
await page.goto(`${BASE}/inventory/items`, { waitUntil: "domcontentloaded" });
await page.getByText("Potato Chips", { exact: false }).first().waitFor({ timeout: 180000 });
await page.waitForTimeout(800);
await page.screenshot({ path: `${OUT}/inventory-items.png` });
console.log("shot: inventory-items");

// Event timeline — rendered only after clicking GENERATE TIMELINE.
await page.goto(`${BASE}/events/1`, { waitUntil: "domcontentloaded" });
await page.getByText("Autumn Dinner Party").first().waitFor({ timeout: 20000 });
const genBtn = page.getByRole("button", { name: /generate timeline/i }).first();
await genBtn.scrollIntoViewIfNeeded();
await genBtn.click();
await page.getByText("start by", { exact: false }).first().waitFor({ timeout: 20000 });
await page.waitForTimeout(800);
await page.screenshot({ path: `${OUT}/event-timeline.png`, fullPage: true });
console.log("shot: event-timeline");

await browser.close();
console.log("done ->", OUT);
