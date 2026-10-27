// Capture LEN-180 Phase-2 proof screenshots against the lena2shots e2e stack.
//
//   node tools/wiki-shots/shot-len180.mjs
//
// Exercises the P1 fixes in a real browser:
//   1. Nav active state on a detail route + admin group visible for admin.
//   2. Notification bell: per-item read, unread dots, manual mark-all.
//   3. Notifications feed page.
//   4. Meal-plan create dialog: weekday select.
//   5. Meal-plan week grid: calendar dates under day names.
//   6. Recipes: Details vs Manage buttons; un-rate; step Position select.
//   7. Grocery list: plan-name header; undo snackbar on line delete.
//   8. Pantry: collapsed Catalog details + UPC helper text.
//   9. Events: 18:00 serve-time default.
//  10. Household: "Find a member" copy + dissolve for sole member.
//  11. Profile: display name field.
//  12. Ask Dot: chips + clear conversation.
//  13. Login: lenaskitchen.app rendered as a link.
//
// Env: SHOTS_OUT (default ./wiki-shots/len180), SHOTS_BASE, SHOTS_ISSUER.

import { createRequire } from "node:module";
import { mkdirSync } from "node:fs";

const require = createRequire(new URL("../../clients/web/package.json", import.meta.url));
const { chromium, request } = require("playwright");

const OUT = process.env.SHOTS_OUT ?? "wiki-shots/len180";
const BASE = process.env.SHOTS_BASE ?? "http://localhost";
const ISSUER = process.env.SHOTS_ISSUER ?? "http://localhost:8085";

mkdirSync(OUT, { recursive: true });

const ctx = await request.newContext();
async function token(sub, email, name) {
  const r = await ctx.get(
    `${ISSUER}/token?sub=${sub}&email=${encodeURIComponent(email)}&name=${encodeURIComponent(name)}`
  );
  return (await r.json()).id_token;
}
const ADMIN = await token("e2e-user-1", "e2e@example.com", "E2E User");
const MEMBER = await token("e2e-user-3", "e2e-member@example.com", "E2E Member");

const browser = await chromium.launch();
async function session(idToken) {
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
  });
  await context.addInitScript(
    (t) => window.localStorage.setItem("lena_id_token", t),
    idToken
  );
  return context.newPage();
}

async function shot(page, name) {
  await page.waitForTimeout(900);
  await page.screenshot({ path: `${OUT}/${name}.png`, fullPage: false });
  console.log("shot:", name);
}

async function gql(idToken, query, variables) {
  const r = await ctx.post(`${BASE}/graphql`, {
    headers: { authorization: `Bearer ${idToken}` },
    data: { query, variables },
  });
  return r.json();
}

// ------------------------------------------------- 1. nav active on detail route
{
  const page = await session(ADMIN);
  const plans = await gql(ADMIN, "query { mealPlans(pageSize: 1) { items { id } } }");
  const planId = plans.data.mealPlans.items[0].id;
  await page.goto(`${BASE}/member/meal-plans/${planId}`, { waitUntil: "domcontentloaded" });
  await page.getByText("Meal Plan", { exact: false }).first().waitFor({ timeout: 60000 });
  await shot(page, "len180-01-nav-active-detail");
  await page.close();
}

// -------------------------------------------- 2. notification bell per-item read
{
  const page = await session(ADMIN);
  await page.goto(`${BASE}/member`, { waitUntil: "domcontentloaded" });
  await page.getByText("Dashboard", { exact: false }).first().waitFor({ timeout: 60000 });
  const bell = page.getByRole("button", { name: /notification/i }).first();
  await bell.click();
  await page.waitForTimeout(1200);
  await shot(page, "len180-02-bell-per-item-read");
  await page.close();
}

// --------------------------------------------------------- 3. notifications feed
{
  const page = await session(ADMIN);
  await page.goto(`${BASE}/member/notifications`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(2000);
  await shot(page, "len180-03-notifications-feed");
  await page.close();
}

// ------------------------------------------------- 4. meal-plan weekday select
{
  const page = await session(ADMIN);
  await page.goto(`${BASE}/member/meal-plans`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: /create/i }).first().waitFor({ timeout: 60000 });
  await page.getByRole("button", { name: /create/i }).first().click();
  const dlg = page.getByRole("dialog");
  await dlg.waitFor();
  // open the Week Starts On select to show named weekdays
  await dlg.getByRole("combobox", { name: /week starts/i }).click();
  await page.waitForTimeout(700);
  await shot(page, "len180-04-weekday-select");
  await page.keyboard.press("Escape");
  await page.close();
}

// ------------------------------------------------- 5. grid calendar dates
{
  const page = await session(ADMIN);
  const plans = await gql(ADMIN, "query { mealPlans(pageSize: 1) { items { id } } }");
  const planId = plans.data.mealPlans.items[0].id;
  await page.goto(`${BASE}/member/meal-plans/${planId}`, { waitUntil: "domcontentloaded" });
  // Wait for a weekday header to render inside the week grid.
  await page.getByText(/^Sun$|^Mon$|^Tue$|^Wed$|^Thu$|^Fri$|^Sat$/).first().waitFor({ timeout: 60000 });
  await page.waitForTimeout(1500);
  await shot(page, "len180-05-grid-dates");
  await page.close();
}

// ----------------------------------------- 6. recipes details/manage + un-rate
{
  const page = await session(ADMIN);
  await page.goto(`${BASE}/member/recipes`, { waitUntil: "domcontentloaded" });
  await page.getByText("Loading").first().waitFor({ state: "detached", timeout: 60000 }).catch(() => {});
  await page.waitForTimeout(1500);
  await shot(page, "len180-06-recipes-details-manage");
  await page.close();
}

// ---------------------------------------------- 7. grocery undo + plan header
{
  const page = await session(ADMIN);
  const lists = await gql(ADMIN, "query { groceryLists(pageSize: 1) { items { id } } }");
  const listId = lists.data.groceryLists.items[0].id;
  await page.goto(`${BASE}/member/grocery-lists/${listId}`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(2000);
  await shot(page, "len180-07-grocery-plan-name");
  // delete a line item → undo snackbar
  const del = page.getByRole("button", { name: /delete|remove/i }).first();
  if (await del.isVisible().catch(() => false)) {
    await del.click();
    await page.waitForTimeout(1000);
    await shot(page, "len180-07b-grocery-undo-snackbar");
  }
  await page.close();
}

// ------------------------------------------- 8. pantry progressive disclosure
{
  const page = await session(ADMIN);
  await page.goto(`${BASE}/member/inventory/items`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: /create|new|add/i }).first().waitFor({ timeout: 60000 });
  await page.getByRole("button", { name: /create|new|add/i }).first().click();
  const dlg = page.getByRole("dialog");
  await dlg.waitFor();
  await page.waitForTimeout(600);
  await shot(page, "len180-08-pantry-collapsed-catalog");
  await page.close();
}

// -------------------------------------------------- 9. event serve-time default
{
  const page = await session(ADMIN);
  const events = await gql(ADMIN, "query { foodEvents(pageSize: 1) { items { id } } }");
  const evId = events.data.foodEvents.items?.[0]?.id;
  if (evId) {
    await page.goto(`${BASE}/member/events/${evId}`, { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(2000);
    const addDish = page.getByRole("button", { name: /add dish|add recipe/i }).first();
    if (await addDish.isVisible().catch(() => false)) {
      await addDish.click();
      await page.getByRole("dialog").waitFor();
      await page.waitForTimeout(600);
      await shot(page, "len180-09-event-serve-default");
      await page.keyboard.press("Escape");
    }
  }
  await page.close();
}

// --------------------------------------------- 10. household copy + dissolve
{
  const page = await session(MEMBER);
  await page.goto(`${BASE}/member/household`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(2000);
  await shot(page, "len180-10-household-copy");
  await page.close();
}

// ---------------------------------------------------------- 11. profile name
{
  const page = await session(MEMBER);
  await page.goto(`${BASE}/member/profile`, { waitUntil: "domcontentloaded" });
  await page.getByLabel(/display name/i).first().waitFor({ timeout: 60000 }).catch(() => {});
  await page.waitForTimeout(800);
  await shot(page, "len180-11-profile-displayname");
  await page.close();
}

// ------------------------------------------------------------- 12. Ask Dot
{
  const page = await session(MEMBER);
  await page.goto(`${BASE}/member/assistant`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(2000);
  await shot(page, "len180-12-askdot");
  await page.close();
}

// -------------------------------------------------------- 13. login link
{
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();
  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1500);
  await shot(page, "len180-13-login-link");
  await context.close();
}

await browser.close();
console.log("done:", OUT);
