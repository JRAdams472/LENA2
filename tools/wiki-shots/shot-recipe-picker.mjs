// LEN-82 proof: meal-plan slot dialog recipe Autocomplete
import { createRequire } from "node:module";
import { mkdirSync } from "node:fs";

const require = createRequire(new URL("../../clients/web/package.json", import.meta.url));
const { chromium, request } = require("playwright");

const OUT = process.env.SHOTS_OUT ?? "wiki-shots";
const BASE = "http://localhost";
const ISSUER = "http://localhost:8085";
mkdirSync(OUT, { recursive: true });

const ctx = await request.newContext();
const res = await ctx.get(`${ISSUER}/token?sub=e2e-user-1&email=e2e%40example.com&name=E2E%20User`);
const { id_token } = await res.json();

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2 });
await context.addInitScript((t) => window.localStorage.setItem("lena_id_token", t), id_token);
const page = await context.newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("CONSOLE:", m.text().slice(0, 200)); });

const mp = await ctx.post(`${BASE}/graphql`, {
  headers: { Authorization: `Bearer ${id_token}` },
  data: { query: `query { mealPlans(page:1, pageSize:5) { items { id name } } }` },
});
const plans = (await mp.json()).data?.mealPlans?.items ?? [];
const planId = plans[0].id;

await page.goto(`${BASE}/meal-plans/${planId}`, { waitUntil: "networkidle" });
await page.waitForTimeout(1500);

// Open the slot dialog via a populated cell.
await page.getByText("Garlic Butter Pasta").first().click();
await page.waitForSelector("text=Filter by Category", { timeout: 10000 });
await page.waitForTimeout(800);
await page.screenshot({ path: `${OUT}/len82-slot-dialog-picker.png` });

// Type into the Recipe autocomplete to show server-side search.
const recipeBox = page.getByRole("combobox", { name: "Recipe" });
await recipeBox.click();
await recipeBox.fill("ch");
await page.waitForTimeout(1200);
await page.screenshot({ path: `${OUT}/len82-slot-dialog-search.png` });

await browser.close();
console.log("done");
