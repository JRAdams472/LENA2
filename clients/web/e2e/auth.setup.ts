import { test as setup, expect } from "@playwright/test";
import { mkdirSync, writeFileSync } from "fs";
import { dirname } from "path";
import { AUTH_FILE } from "../playwright.config";
import { mintToken, PRIMARY_USER } from "./helpers";

setup("authenticate as the e2e user", async ({ browser, request, baseURL }) => {
  const token = await mintToken(request, PRIMARY_USER);

  const context = await browser.newContext();
  // The app stores the token in sessionStorage (per-tab) and migrates any
  // localStorage copy on load. context.storageState() does not persist
  // sessionStorage, so seed localStorage — the app moves it on first load.
  await context.addInitScript(
    (t) => window.localStorage.setItem("lena_id_token", t),
    token
  );
  const page = await context.newPage();
  await page.goto("/");

  // The authenticated layout shows the signed-in user's email.
  await expect(
    page.getByText(PRIMARY_USER.email).first()
  ).toBeVisible();
  await context.close();

  // Write the storage state explicitly: by now the app has migrated the
  // token out of the live page's localStorage, but the dependent project
  // still needs the seed to be present in the saved file.
  mkdirSync(dirname(AUTH_FILE), { recursive: true });
  writeFileSync(
    AUTH_FILE,
    JSON.stringify({
      cookies: [],
      origins: [
        {
          origin: baseURL ?? "http://localhost",
          localStorage: [{ name: "lena_id_token", value: token }],
        },
      ],
    })
  );
});
