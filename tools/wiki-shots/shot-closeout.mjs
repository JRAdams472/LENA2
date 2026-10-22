// LEN-168 close-out shots: refreshed wiki captures for screens changed by
// the deep-review web phase.
//
//   node tools/wiki-shots/shot-closeout.mjs
//
// Requires the lena2shots stack up and tools/wiki-shots/seed_demo.py run.

import { createRequire } from "node:module";
import { mkdirSync } from "node:fs";

const require = createRequire(new URL("../../clients/web/package.json", import.meta.url));
const { chromium, request } = require("playwright");

const OUT = process.env.SHOTS_OUT ?? "wiki-shots";
const BASE = process.env.SHOTS_BASE ?? "http://localhost";
const ISSUER = process.env.SHOTS_ISSUER ?? "http://localhost:8085";
const API = process.env.SHOTS_API ?? "http://localhost/graphql";

mkdirSync(OUT, { recursive: true });

const ctx = await request.newContext();
const res = await ctx.get(
  `${ISSUER}/token?sub=e2e-user-1&email=e2e%40example.com&name=E2E%20User`
);
const { id_token } = await res.json();

async function gql(query, variables = {}) {
  const r = await ctx.post(API, {
    data: { query, variables },
    headers: { authorization: `Bearer ${id_token}` },
  });
  return (await r.json()).data;
}

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
});
await context.addInitScript(
  (t) => window.localStorage.setItem("lena_id_token", t),
  id_token
);
const page = await context.newPage();

// --- Meal plan week view -----------------------------------------------
const plans = await gql(
  "{ mealPlans(page:1,pageSize:50){ items { id name } } }"
);
const plan = plans.mealPlans.items.find((p) => p.name.startsWith("Week of"))
  ?? plans.mealPlans.items[0];
await page.goto(`${BASE}/member/meal-plans/${plan.id}`, {
  waitUntil: "domcontentloaded",
});
await page.getByText(plan.name, { exact: false }).first().waitFor({ timeout: 60000 });
await page.waitForTimeout(1500);
await page.screenshot({ path: `${OUT}/meal-plan-week.png`, fullPage: false });
console.log("shot: meal-plan-week");

// --- Items page: edit dialog with brand/category pickers ----------------
await page.goto(`${BASE}/member/inventory/items`, { waitUntil: "domcontentloaded" });
await page.waitForTimeout(2500);
// Open the first row's edit action (DataTable tags it data-testid).
await page
  .locator("table tbody tr")
  .first()
  .locator('[data-testid="row-edit-button"]')
  .click();
await page.getByRole("dialog").waitFor({ timeout: 15000 });
// Brand and Category are comboboxes inside the dialog post-P6.
await page
  .getByRole("dialog")
  .getByRole("combobox")
  .first()
  .waitFor({ timeout: 15000 });
await page.waitForTimeout(800);
await page.screenshot({ path: `${OUT}/inventory-item-edit.png`, fullPage: false });
console.log("shot: inventory-item-edit");

await browser.close();
