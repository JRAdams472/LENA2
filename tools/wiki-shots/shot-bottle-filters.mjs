// LEN-83 proof: bottles page filters hit one server-side paged query.
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

// Favorite a bottle so the favorites toggle has something to show.
await ctx.post(`${BASE}/graphql`, {
  headers: { Authorization: `Bearer ${id_token}` },
  data: { query: `mutation { setBottleFavorite(bottleId: "1", isFavorite: true) { id } }` },
});

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2 });
await context.addInitScript((t) => window.localStorage.setItem("lena_id_token", t), id_token);
const page = await context.newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("CONSOLE:", m.text().slice(0, 200)); });

const gqlCalls = [];
page.on("request", (r) => {
  if (r.url().includes("/graphql") && r.method() === "POST") {
    const body = r.postData() ?? "";
    if (body.includes("bottles(")) gqlCalls.push(body.slice(0, 500));
  }
});

// Filter selects are unlabelled comboboxes; DOM order = Country, Region, Type, Vintage.
const select = (i) => page.locator("div[role=combobox]").nth(i);

await page.goto(`${BASE}/wine/bottles`, { waitUntil: "networkidle" });
await page.waitForTimeout(1500);
await page.screenshot({ path: `${OUT}/len83-bottles-default.png` });
const defaultCalls = gqlCalls.length;

// Country filter -> one filtered request; pagination control stays.
gqlCalls.length = 0;
await select(0).click();
await page.waitForTimeout(400);
const countryOpt = page.getByRole("option", { name: "France" });
console.log("country option picked:", await countryOpt.textContent());
await countryOpt.click();
await page.waitForTimeout(1200);
await page.screenshot({ path: `${OUT}/len83-bottles-country-filter.png` });
console.log("default-view bottles queries:", defaultCalls);
console.log("country-filter bottles queries:", gqlCalls.length);
console.log("pagination visible:", await page.getByText(/Page \d+ of \d+/).count());

// Favorites toggle -> one favorites-only request.
gqlCalls.length = 0;
await page.getByText("Favorites").click();
await page.waitForTimeout(1200);
await page.screenshot({ path: `${OUT}/len83-bottles-favorites.png` });
console.log("favorites bottles queries:", gqlCalls.length);
console.log("favorites query vars:", gqlCalls[0]?.match(/"variables":\{[^}]*\}/)?.[0]);

await browser.close();
