import { test, expect } from "@playwright/test";
import { graphql, mintToken, PRIMARY_USER } from "./helpers";

const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost";

// LEN-31: the medical/allergen disclaimer — public page plus the consent
// gate in front of allergy recording.
test("disclaimer page is public and linked from the shell", async ({
  page,
}) => {
  await page.goto("/disclaimer");
  await expect(
    page.getByRole("heading", { name: /medical & allergen disclaimer/i })
  ).toBeVisible();
  await expect(
    page.getByText(/not medical, nutritional, or dietary advice/i)
  ).toBeVisible();
  await expect(
    page.getByText(/Lena's Kitchen accepts no liability/i)
  ).toBeVisible();
  await expect(
    page.getByText(/check the product's actual label/i)
  ).toBeVisible();
});

// A disposable identity starts unaccepted: recording is refused with the
// DISCLAIMER_REQUIRED extension until acceptDisclaimer runs.
test("setMyAllergy is gated until the disclaimer is accepted", async ({
  request,
}) => {
  const tag = Date.now().toString(36);
  const token = await mintToken(request, {
    sub: `e2e-disc-${tag}`,
    email: `e2e-disc-${tag}@example.com`,
    name: "E2E Disclaimer",
  });

  const res = await request.post(`${BASE_URL}/graphql`, {
    headers: { Authorization: `Bearer ${token}` },
    data: {
      query: `mutation { setMyAllergy(allergenId: "1", kind: allergy, on: true) }`,
    },
  });
  const payload = (await res.json()) as {
    errors?: { message: string; extensions?: { code?: string } }[];
  };
  expect(payload.errors?.[0]?.extensions?.code).toBe("DISCLAIMER_REQUIRED");

  await graphql(request, token, `mutation { acceptDisclaimer }`);
  const me = await graphql<{ me: { disclaimerAccepted: boolean } }>(
    request,
    token,
    `{ me { disclaimerAccepted } }`
  );
  expect(me.me.disclaimerAccepted).toBe(true);
});

// The full UI flow on a fresh identity: the allergy toggle opens the
// consent dialog instead of writing, and accepting it retries the pick.
test("profile allergy picker routes through consent", async ({
  browser,
  request,
}) => {
  const tag = Date.now().toString(36);
  const token = await mintToken(request, {
    sub: `e2e-disc-ui-${tag}`,
    email: `e2e-disc-ui-${tag}@example.com`,
    name: "E2E Disclaimer UI",
  });

  const context = await browser.newContext({ baseURL: BASE_URL });
  await context.addInitScript(
    (t) => window.localStorage.setItem("lena_id_token", t),
    token
  );
  const page = await context.newPage();
  try {
    await page.goto("/member/profile");
    await expect(page.getByText(/Allergies & dietary restrictions/i)).toBeVisible();

    const group = page.locator("[data-testid^='allergy-kind-']").first();
    await expect(group).toBeVisible();
    await group.getByRole("button", { name: "Dietary" }).click();

    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByText(/not medical, nutritional, or dietary advice/i)
    ).toBeVisible();
    await dialog.getByTestId("disclaimer-accept").click();
    await expect(dialog).not.toBeVisible();

    // The pending pick retries after acceptance.
    await expect(group.getByRole("button", { name: "Dietary" })).toHaveAttribute(
      "aria-pressed",
      "true"
    );

    // Consent is one-time — the next change writes directly.
    await group.getByRole("button", { name: "None" }).click();
    await expect(
      group.getByRole("button", { name: "None" })
    ).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByRole("dialog")).not.toBeVisible();
  } finally {
    await context.close();
  }
});
