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
import { execSync } from "child_process";

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
const pastaId = await recipeId("Garlic Butter Pasta");

async function shot(name, path, waitFor) {
  // NOTE: "networkidle" never settles — the app polls notifications.
  await page.goto(`${BASE}${path}`, { waitUntil: "domcontentloaded" });
  if (waitFor) {
    try {
      await page.getByText(waitFor, { exact: false }).first().waitFor({ timeout: 60000 });
    } catch (e) {
      console.log("shot failed:", name, "url:", page.url());
      console.log((await page.locator("body").innerText()).slice(0, 800));
      throw e;
    }
  }
  await page.waitForTimeout(1200);
  await page.screenshot({ path: `${OUT}/${name}.png`, fullPage: false });
  console.log("shot:", name);
}

// Seed an expiring pantry item and fire the notification sweep so the
// bell shot shows a real reminder.
async function gql(query, variables = {}) {
  const r = await ctx.post(`${BASE}/graphql`, {
    headers: { Authorization: `Bearer ${id_token}` },
    data: { query, variables },
  });
  const body = await r.json();
  if (body.errors) throw new Error(JSON.stringify(body.errors));
  return body.data;
}
const expiringItem = await gql(
  `query { items(page: 1, pageSize: 25) { items { id name } } }`
).then(
  (d) =>
    d.items.items.find((i) => /milk/i.test(i.name)) ?? d.items.items[0]
);
await gql(
  `mutation ($itemId: ID!, $expiresAt: Time) {
    adjustUserItem(itemId: $itemId, quantity: 3, expiresAt: $expiresAt) { id }
  }`,
  {
    itemId: expiringItem.id,
    expiresAt: new Date(Date.now() + 2 * 86400_000).toISOString(),
  }
);
await gql(`mutation { triggerNotificationSweep }`);

// Restock suggestions: drop a few stocked items below a minimum and give
// them real selection engagement so suggestedRestockItems is non-empty.
for (const kw of ["milk", "flour", "butter"]) {
  const hit = await gql(
    `query ($s: String) { items(page: 1, pageSize: 5, search: $s) { items { id name } } }`,
    { s: kw }
  ).then((d) => d.items.items.find((i) => new RegExp(kw, "i").test(i.name)));
  if (!hit) continue;
  await gql(
    `mutation ($itemId: ID!, $minQty: Float) {
      adjustUserItem(itemId: $itemId, quantity: 0, minQty: $minQty) { id }
    }`,
    { itemId: hit.id, minQty: 2 }
  );
  await gql(
    `mutation ($id: ID!) { recordSelection(entityType: item, entityId: $id) }`,
    { id: hit.id }
  );
}

await shot("dashboard", "/", "Time to restock");
await shot("recipes", "/recipes", "Herb Roast Chicken");
await shot("recipe-detail", `/recipes/${roastId}`, "Herb Roast Chicken");
// Ingredient-keyed lines + preferred brand — the pasta recipe carries both.
// Element-shot the Ingredients paper; it's below the categories panel and
// scrollIntoView is unreliable on this page.
await page.goto(`${BASE}/recipes/${pastaId}`, { waitUntil: "domcontentloaded" });
const ingHeading = page.getByRole("heading", { name: "Ingredients" });
await ingHeading.waitFor();
await ingHeading
  .locator("xpath=ancestor::div[contains(@class,'MuiPaper')][1]")
  .screenshot({ path: `${OUT}/recipe-detail-ingredients.png` });
console.log("shot: recipe-detail-ingredients");
await shot("recipe-categories-admin", "/recipes/categories", "Cuisine");
await shot("notification-settings", "/notifications", "Expiring pantry items");
await shot("inventory-categories", "/inventory/categories", "Meat");

// Notification bell open with the seeded expiry reminder.
await page.goto(`${BASE}/`, { waitUntil: "domcontentloaded" });
await page.getByText("Garlic Butter Pasta").first().waitFor({ timeout: 20000 });
await page.getByLabel("notifications").click();
await page.getByText("expires soon", { exact: false }).first().waitFor({ timeout: 10000 });
await page.waitForTimeout(400);
await page.screenshot({ path: `${OUT}/notification-bell.png` });
await page.keyboard.press("Escape");
console.log("shot: notification-bell");

// Category filter in action — open the Cuisine dropdown on the recipes list.
await page.goto(`${BASE}/recipes`, { waitUntil: "domcontentloaded" });
await page.getByText("Herb Roast Chicken").first().waitFor({ timeout: 20000 });
await page.getByRole("combobox", { name: "Cuisine" }).click();
await page.getByRole("option", { name: "Italian" }).waitFor({ timeout: 10000 });
await page.waitForTimeout(400);
await page.screenshot({ path: `${OUT}/recipe-category-filter.png` });
await page.keyboard.press("Escape");
console.log("shot: recipe-category-filter");

// Semantic mode — toggle the switch and search by vibe instead of name.
await page.goto(`${BASE}/recipes`, { waitUntil: "domcontentloaded" });
await page.getByText("Herb Roast Chicken").first().waitFor({ timeout: 20000 });
await page.getByText("Semantic", { exact: true }).click();
await page.getByRole("textbox", { name: "Search" }).fill("cozy comfort dinner");
await page.waitForTimeout(1500);
await page.screenshot({ path: `${OUT}/recipes-semantic.png` });
console.log("shot: recipes-semantic");
await shot("meal-plans", "/meal-plans", "Week of");
const planId = await gql(
  `query { mealPlans(page: 1, pageSize: 1) { items { id } } }`
).then((d) => d.mealPlans.items[0].id);
await shot("meal-plan-week", `/meal-plans/${planId}`, "Week of");
await shot("grocery-lists", "/grocery-lists", "20");
// Prefer the seeded list with a store set so the shot shows route groups.
const listId = await gql(
  `query { groceryLists(page: 1, pageSize: 10) { items { id store { id } } } }`
).then(
  (d) =>
    (d.groceryLists.items.find((l) => l.store != null) ??
      d.groceryLists.items[0]).id
);
await shot("grocery-list", `/grocery-lists/${listId}`, "Time to restock");

// First-time check-off of an ingredient-only line opens the brand picker —
// shoot the dialog, then cancel so the row stays unchecked.
await page.goto(`${BASE}/grocery-lists/${listId}`, { waitUntil: "domcontentloaded" });
await page.getByText("olive oil", { exact: false }).first().waitFor({ timeout: 30000 });
const oliveRow = page
  .locator("div")
  .filter({ has: page.getByRole("checkbox") })
  .filter({ hasText: "olive oil" })
  .last();
await oliveRow.getByRole("checkbox").click();
await page.getByText("did you buy", { exact: false }).first().waitFor({ timeout: 15000 });
await page.waitForTimeout(600);
await page.screenshot({ path: `${OUT}/grocery-brand-picker.png` });
console.log("shot: grocery-brand-picker");
await page.keyboard.press("Escape");
await shot("ingredients-admin", "/inventory/ingredients", "garlic");

// Allergen admin pages. Member records + curated item flags come from
// seed_demo.py; the suggestion queue has no non-AI write path and the e2e
// provider returns canned text, so seed one pending proposal via SQL for
// the queue shot.
execSync("docker exec -i lena2shots-db-1 psql -U lena -d lena", {
  input: `INSERT INTO inventory.allergen_suggestion
    (recipe_id, target_kind, ingredient_id, allergen_id, kind, rationale, created_by)
  SELECT r.recipe_id, 'ingredient', g.ingredient_id, a.allergen_id, 'may_contain',
         'Ingredient may carry sulfites from processing or packaging.',
         'e2e@example.com'
  FROM recipe.recipe r, inventory.ingredient g, inventory.allergen a
  WHERE r.name = 'Garlic Butter Pasta' AND g.name = 'garlic' AND a.name = 'sulfites'
  ON CONFLICT DO NOTHING;`,
});
await shot("allergen-registry", "/inventory/allergens", "Milk");
await shot(
  "allergen-suggestions",
  "/inventory/allergen-suggestions",
  "Review queue"
);
// Element-shot the allergy card so the tri-state toggles (with the seeded
// milk allergy selected) fill the frame.
await page.goto(`${BASE}/profile`, { waitUntil: "domcontentloaded" });
const allergyHeading = page.getByRole("heading", {
  name: "Allergies & dietary restrictions",
});
await allergyHeading.waitFor({ timeout: 30000 });
await allergyHeading
  .locator("xpath=ancestor::div[contains(@class,'MuiPaper')][1]")
  .screenshot({ path: `${OUT}/profile-allergies.png` });
console.log("shot: profile-allergies");
// The pasta recipe carries flagged items (butter/cheese → milk, pasta →
// wheat) and the e2e admin records a milk allergy, so the detail page
// renders the warning chip.
await shot(
  "recipe-allergy-warning",
  `/recipes/${pastaId}`,
  "milk (allergy; contains)"
);

await shot("events", "/events", "Autumn Dinner Party");
await shot("event-detail", "/events/1", "Autumn Dinner Party");
await shot("wine-bottles", "/wine/bottles", "Silver Oak");
await shot("household", "/household", "E2E Member");
// The items page is server-paginated — wait for the first page's rows
// rather than a specific catalog name.
await page.goto(`${BASE}/inventory/items`, { waitUntil: "domcontentloaded" });
await page.getByText("ADD TO INVENTORY", { exact: false }).first().waitFor({ timeout: 120000 });
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

// AI assistant — the e2e overlay runs LENA_AI_PROVIDER=mock, so the page
// is live and Ask returns the canned deterministic answer.
await shot("assistant", "/assistant", "Ask Dot");
await page.getByPlaceholder("Ask Dot").fill("What's in my wine cellar?");
await page.getByRole("button", { name: /ask|send/i }).first().click();
await page.getByText("mock provider", { exact: false }).first().waitFor({ timeout: 30000 });
await page.waitForTimeout(400);
await page.screenshot({ path: `${OUT}/assistant-answer.png`, fullPage: false });
console.log("shot: assistant-answer");

// Signed-out login screen for Getting-Started — a fresh context with no
// seeded token shows the real sign-in buttons.
const anon = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
});
const loginPage = await anon.newPage();
await loginPage.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
await loginPage.getByText("Sign in", { exact: false }).first().waitFor({ timeout: 20000 });
await loginPage.waitForTimeout(800);
await loginPage.screenshot({ path: `${OUT}/login.png` });
console.log("shot: login");
await anon.close();

await browser.close();
console.log("done ->", OUT);
