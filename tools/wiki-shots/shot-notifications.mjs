// One-off: notification settings page with the new push toggles.
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

// Turn on push for _all + one category so both states are visible.
const gql = (q, v = {}) =>
  ctx.post(`${BASE}/graphql`, {
    headers: { Authorization: `Bearer ${id_token}` },
    data: { query: q, variables: v },
  });
await gql(`mutation { setNotificationCategoryPushEnabled(category: "_all", enabled: true) }`);
await gql(`mutation { setNotificationCategoryPushEnabled(category: "household", enabled: true) }`);

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
await page.goto(`${BASE}/notifications`, { waitUntil: "domcontentloaded" });
await page
  .getByRole("heading", { name: "Notification settings", exact: false })
  .first()
  .waitFor({ timeout: 60000 });
await page.waitForTimeout(1200);
await page.screenshot({ path: `${OUT}/notification-settings.png` });
console.log("shot: notification-settings");
await browser.close();
