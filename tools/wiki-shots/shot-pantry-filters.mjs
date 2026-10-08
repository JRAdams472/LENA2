// LEN-84 proof: items page pantry filters hit one server-side paged query.
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

// Seed a favorite so the favorites toggle has something to show.
const itemsRes = await ctx.post(`${BASE}/graphql`, {
  headers: { Authorization: `Bearer ${id_token}` },
  data: { query: `{ userItems(page: 1, pageSize: 1, inStock: true) { items { item { id } } } }` },
});
const first = (await itemsRes.json()).data?.userItems?.items?.[0]?.item?.id;
if (first) {
  await ctx.post(`${BASE}/graphql`, {
    headers: { Authorization: `Bearer ${id_token}` },
    data: { query: `mutation { setItemFavorite(itemId: "${first}", isFavorite: true) { id } }` },
  });
}

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2 });
await context.addInitScript((t) => window.localStorage.setItem("lena_id_token", t), id_token);
const page = await context.newPage();
page.on("console", (m) => { if (m.type() === "error") console.log("CONSOLE:", m.text().slice(0, 200)); });

const gqlCalls = [];
page.on("request", (r) => {
  if (r.url().endsWith("/graphql") && r.method() === "POST") {
    const q = JSON.parse(r.postData() ?? "{}").query ?? "";
    gqlCalls.push(q.replace(/\s+/g, " ").slice(0, 120));
  }
});

await page.goto(`${BASE}/inventory/items`, { waitUntil: "networkidle" });
await page.waitForTimeout(600);

// 1. In Stock toggle — one userItems request with inStock: true
gqlCalls.length = 0;
await page.getByLabel("In Stock").click();
await page.waitForTimeout(1200);
console.log("inStock calls:", gqlCalls.length, gqlCalls.filter((q) => q.includes("userItems")));
await page.screenshot({ path: `${OUT}/pantry-instock.png` });

// 2. + Favorites — composed filters in one request
gqlCalls.length = 0;
await page.getByLabel("Favorites").click();
await page.waitForTimeout(1200);
console.log("fav calls:", gqlCalls.length, gqlCalls.filter((q) => q.includes("userItems")));
await page.screenshot({ path: `${OUT}/pantry-instock-favorites.png` });

// 3. Search term — still one filtered request
gqlCalls.length = 0;
await page.getByLabel("Search").fill("milk");
await page.waitForTimeout(1600);
console.log("search calls:", gqlCalls.length, gqlCalls.filter((q) => q.includes("userItems")));
await page.screenshot({ path: `${OUT}/pantry-filtered-search.png` });

await browser.close();
console.log("done");
