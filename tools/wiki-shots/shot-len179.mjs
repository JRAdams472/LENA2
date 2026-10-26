// Capture LEN-179 Phase-1 proof screenshots against the lena2shots e2e stack.
//
//   node tools/wiki-shots/shot-len179.mjs
//
// Exercises each P0 fix in a real browser:
//   1. Silent save failures — dialog stays open + shows the mutation error
//      (mutation is route-intercepted to fail deterministically).
//   2. Server-side sort — clicking a column header sends sort/sortDesc
//      args to GraphQL (request body is printed for the run log).
//   3. Slot editor — real MUI Dialog, keyboard-openable cell, dirty-Escape
//      discard confirmation.
//   4. Category change — picker dialog (no window.prompt).
//   5. Member store creation — "+ New store…" in the grocery store select.
//   6. Member wine submission — "Submit new bottle" pending flow, plus the
//      admin pending-bottles review section.
//
// Env: SHOTS_OUT (default ./wiki-shots/len179), SHOTS_BASE, SHOTS_ISSUER.

import { createRequire } from "node:module";
import { mkdirSync } from "node:fs";

const require = createRequire(new URL("../../clients/web/package.json", import.meta.url));
const { chromium, request } = require("playwright");

const OUT = process.env.SHOTS_OUT ?? "wiki-shots/len179";
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

// ---------------------------------------------------------------- 1. silent save
{
  const page = await session(ADMIN);
  // Fail createMealPlan deterministically — proves the dialog stays open and
  // surfaces the error instead of silently closing.
  await page.route("**/graphql", (route) => {
    const body = route.request().postDataJSON();
    if (body?.query?.includes("createMealPlan")) {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          errors: [{ message: "simulated failure — plan name already exists" }],
        }),
      });
    }
    return route.continue();
  });
  await page.goto(`${BASE}/member/meal-plans`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: /create/i }).first().waitFor({ timeout: 60000 });
  await page.getByRole("button", { name: /create/i }).first().click();
  const dlg = page.getByRole("dialog");
  await dlg.getByLabel(/name/i).first().fill("Proof Plan");
  await dlg.getByRole("button", { name: "Save" }).click();
  await dlg.getByRole("alert").waitFor({ timeout: 15000 });
  await shot(page, "len179-01-save-error-stays-open");
  await page.unroute("**/graphql");
  await page.context().close();
}

// ---------------------------------------------------------------- 2. server sort
{
  const page = await session(ADMIN);
  const sortCalls = [];
  page.on("request", (req) => {
    if (!req.url().includes("graphql")) return;
    const body = req.postDataJSON();
    if (body?.variables && Object.keys(body.variables).some((k) => k.startsWith("sort"))) {
      sortCalls.push(body.variables);
    }
  });
  await page.goto(`${BASE}/member/recipes`, { waitUntil: "domcontentloaded" });
  const nameHeader = page.getByRole("columnheader", { name: /name/i });
  await nameHeader.waitFor({ timeout: 60000 });
  // The clickable control is the TableSortLabel inside the header cell.
  await nameHeader.locator(".MuiTableSortLabel-root").click();
  await page.waitForTimeout(1500);
  const sorted = sortCalls.filter((v) => v.sort);
  console.log("sort request variables:", JSON.stringify(sorted.at(-1)));
  if (!sorted.length) throw new Error("no sort request captured");
  await shot(page, "len179-02-server-sort-name");
  await page.context().close();
}

// ------------------------------------------------- 3. slot dialog + dirty escape
{
  const page = await session(ADMIN);
  await page.goto(`${BASE}/member/meal-plans/2`, { waitUntil: "domcontentloaded" });
  const cell = page.getByRole("button", { name: /— add a meal/i }).first();
  await cell.waitFor({ timeout: 60000 });
  // Keyboard-open the cell — proves the role=button + Enter keydown path.
  await cell.focus();
  await page.keyboard.press("Enter");
  const dlg = page.getByRole("dialog");
  await dlg.waitFor({ timeout: 15000 });
  await shot(page, "len179-03-slot-dialog");
  // Dirty the form via a tracked field (servings is in the signature),
  // then press Escape → discard confirmation.
  await dlg.getByLabel("Servings").fill("3");
  await page.waitForTimeout(500);
  await page.keyboard.press("Escape");
  await page.getByText("Discard changes?").waitFor({ timeout: 10000 });
  await shot(page, "len179-04-slot-discard-confirm");
  await page.context().close();
}

// ---------------------------------------------------------- 4. category picker
{
  const page = await session(ADMIN);
  await page.goto(`${BASE}/member/inventory/items`, { waitUntil: "domcontentloaded" });
  const catBtn = page.getByRole("button", { name: "Category" }).first();
  await catBtn.waitFor({ timeout: 60000 });
  await catBtn.click();
  const dlg = page.getByRole("dialog", { name: /change .* category/i });
  await dlg.waitFor({ timeout: 15000 });
  await dlg.getByRole("combobox").click();
  await page.waitForTimeout(800);
  await shot(page, "len179-05-category-picker");
  await page.context().close();
}

// ------------------------------------------------------------ 5. create a store
{
  const page = await session(ADMIN);
  await page.goto(`${BASE}/member/grocery-lists/2`, { waitUntil: "domcontentloaded" });
  await page.getByLabel("Store").waitFor({ timeout: 60000 });
  await page.getByLabel("Store").click();
  await page.getByRole("option", { name: /new store/i }).click();
  const dlg = page.getByRole("dialog", { name: /new store/i });
  await dlg.waitFor({ timeout: 15000 });
  await shot(page, "len179-06-new-store-dialog");
  await dlg.getByLabel(/store name/i).fill(`ProofMart ${Date.now() % 10000}`);
  await dlg.getByRole("button", { name: "Save" }).click();
  await dlg.waitFor({ state: "detached", timeout: 15000 });
  await shot(page, "len179-07-store-selected");
  await page.context().close();
}

// ----------------------------------------------------- 6. member submit bottle
{
  const page = await session(MEMBER);
  await page.goto(`${BASE}/member/wine/bottles`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: /create/i }).first().waitFor({ timeout: 60000 });
  await page.getByRole("button", { name: /create/i }).first().click();
  const cellarDlg = page.getByRole("dialog", { name: /add bottle to cellar/i });
  await cellarDlg.waitFor({ timeout: 15000 });
  const bottleField = cellarDlg.getByRole("combobox", { name: "Bottle" });
  await bottleField.click();
  await bottleField.fill("zzznomatchvine");
  await page.waitForTimeout(1200);
  await page.getByText("Submit new bottle").click();
  const submitDlg = page.getByRole("dialog", { name: /submit new bottle/i });
  await submitDlg.waitFor({ timeout: 15000 });
  await shot(page, "len179-08-submit-bottle-dialog");
  // Pickers: type, country (France — seeded with the Bordeaux region),
  // then region (scoped to country), vintage. While a Select's menu is
  // open it acts as a modal and aria-hides the dialog, so wait for each
  // listbox to detach before opening the next picker.
  const pick = async (label, optionText) => {
    await submitDlg.getByRole("combobox", { name: label }).click();
    const opt = optionText
      ? page.locator("li[role='option']", { hasText: optionText }).first()
      : page.locator("li[role='option']").first();
    await opt.waitFor({ timeout: 10000 });
    await opt.click();
    await page.locator("[role='listbox']").waitFor({ state: "detached", timeout: 10000 });
  };
  await pick("Type");
  await pick("Country", "France");
  await pick("Region", "Bordeaux");
  await pick("Vintage");
  await submitDlg.getByLabel("Vineyard").fill("E2E Submit Vine");
  await submitDlg.getByRole("button", { name: "Save" }).click();
  await page.getByText(/submitted for review/i).waitFor({ timeout: 15000 });
  await shot(page, "len179-09-submit-bottle-pending");
  await page.context().close();
}

// ------------------------------------------------- admin pending-bottles review
{
  const page = await session(ADMIN);
  await page.goto(`${BASE}/member/items/pending`, { waitUntil: "domcontentloaded" });
  await page.getByText(/pending bottles/i).waitFor({ timeout: 60000 });
  await shot(page, "len179-10-admin-pending-bottles");
  // Approve the just-submitted bottle so the demo data stays clean.
  const row = page.getByRole("row", { name: /e2e submit vine/i });
  if (await row.count()) {
    await row.getByRole("button", { name: /approve/i }).click();
    await page.waitForTimeout(1200);
    console.log("approved E2E Submit Vine");
  }
  await page.context().close();
}

await browser.close();
console.log("done →", OUT);
