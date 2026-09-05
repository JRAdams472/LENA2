import { test as setup, expect } from "@playwright/test";
import { AUTH_FILE } from "../playwright.config";
import { mintToken, PRIMARY_USER } from "./helpers";

setup("authenticate as the e2e user", async ({ browser, request }) => {
  const token = await mintToken(request, PRIMARY_USER);

  const context = await browser.newContext();
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

  await context.storageState({ path: AUTH_FILE });
  await context.close();
});
