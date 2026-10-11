// LEN-139 proof: manual grocery list creation + source labels.
import { createRequire } from "node:module";
import { mkdirSync } from "node:fs";

const require = createRequire(new URL("../../clients/web/package.json", import.meta.url));
const { chromium, request } = require("playwright");

const OUT = process.env.SHOTS_OUT ?? "C:/tmp/len139";
const BASE = "http://localhost";
const ISSUER = "http://localhost:8085";
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
await context.addInitScript(
  (t) => window.localStorage.setItem("lena_id_token", t),
  id_token
);
const page = await context.newPage();

// Resolve a meal plan name/id from the seeded data.
const plansRes = await ctx.post(`${BASE}/graphql`, {
  headers: { Authorization: `Bearer ${id_token}` },
  data: { query: `query { mealPlans(page: 1, pageSize: 10) { items { id name } } }` },
});
const plans = (await plansRes.json()).data?.mealPlans?.items ?? [];
console.log("plans:", JSON.stringify(plans));

await page.goto(`${BASE}/member/grocery-lists`, { waitUntil: "domcontentloaded" });
await page.getByRole("button", { name: /create/i }).waitFor({ timeout: 15000 });
await page.waitForTimeout(1500);

// 1. Create dialog with the plan dropdown open.
await page.getByRole("button", { name: /create/i }).click();
await page.getByRole("dialog").waitFor();
await page.getByLabel("Meal plan").click();
await page.waitForTimeout(400);
await page.screenshot({ path: `${OUT}/grocery-create-dialog.png` });

// 2. Pick the manual option and save — should land on an empty list.
await page.getByRole("option", { name: /manual list/i }).click();
await page.getByRole("button", { name: "Save" }).click();
await page.waitForURL(/\/member\/grocery-lists\/\d+/, { timeout: 10000 });
await page.waitForTimeout(1000);
await page.screenshot({ path: `${OUT}/grocery-manual-detail.png` });
console.log("manual list url:", page.url());

// 3. Back to lists — the new row should say "Manual list".
await page.goto(`${BASE}/member/grocery-lists`, { waitUntil: "domcontentloaded" });
await page.getByText("Manual list").first().waitFor({ timeout: 10000 });
await page.waitForTimeout(500);

// 4. Generate one from the seeded plan to prove the old path still works.
if (plans.length) {
  await page.getByRole("button", { name: /create/i }).click();
  await page.getByRole("dialog").waitFor();
  await page.getByLabel("Meal plan").click();
  await page.getByRole("option", { name: plans[0].name }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await page.waitForURL(/\/member\/grocery-lists\/\d+/, { timeout: 10000 });
  await page.goto(`${BASE}/member/grocery-lists`, { waitUntil: "domcontentloaded" });
  await page.getByText(plans[0].name).first().waitFor({ timeout: 10000 });
  await page.waitForTimeout(500);
}
await page.screenshot({ path: `${OUT}/grocery-lists-rows.png` });
console.log("done");
await browser.close();
