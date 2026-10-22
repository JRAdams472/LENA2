// Focused proof shot for LEN-174: /member/wine/bottles must render the
// household cellar overlay (qty + location) merged onto catalog bottles.
//
//   node tools/wiki-shots/shot-wine-cellar.mjs
//
// Requires the lena2shots stack up and tools/wiki-shots/seed_demo.py run.

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
const context = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
});
await context.addInitScript(
  (t) => window.localStorage.setItem("lena_id_token", t),
  id_token
);
const page = await context.newPage();

await page.goto(`${BASE}/member/wine/bottles`, { waitUntil: "domcontentloaded" });
// "Cellar rack A" only renders when the userBottles overlay lands on the
// catalog row — it is proof of the join, not just the page loading.
await page.getByText("Cellar rack A", { exact: false }).first().waitFor({ timeout: 60000 });
await page.getByText("Silver Oak", { exact: false }).first().waitFor({ timeout: 10000 });
await page.waitForTimeout(1200);
await page.screenshot({ path: `${OUT}/wine-bottles.png`, fullPage: false });
console.log("shot: wine-bottles");

await browser.close();
