import { test, expect } from "@playwright/test";
import { graphql, mintToken, PRIMARY_USER } from "./helpers";

test("profile danger zone gates the irreversible delete flow", async ({
  page,
}) => {
  await page.goto("/member/profile");
  await expect(page.getByText("Danger zone")).toBeVisible();

  await page.getByTestId("delete-account-open").click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText(/irreversible/)).toBeVisible();
  await expect(dialog.getByText(/Type DELETE to confirm/)).toBeVisible();

  const submit = dialog.getByTestId("delete-account-submit");
  await expect(submit).toBeDisabled();
  await dialog.getByTestId("delete-account-confirm-input").fill("DELET");
  await expect(submit).toBeDisabled();
  await dialog.getByTestId("delete-account-confirm-input").fill("DELETE");
  await expect(submit).toBeEnabled();

  // Cancel leaves the account — and the session — untouched.
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByText("Profile").first()).toBeVisible();
});

// The real deletion runs against a disposable identity minted for this
// run so the seeded e2e users survive for the other specs. The shots
// stack auto-approves new accounts (LENA_REQUIRE_MEMBER_APPROVAL=false),
// and a provider token re-provisions after deletion — so the proof of
// removal is that `me` afterwards resolves to a *different* user id.
test("deleteMyAccount removes a real account end to end", async ({
  request,
}) => {
  const tag = Date.now().toString(36);
  const doomed = {
    sub: `e2e-delete-${tag}`,
    email: `e2e-delete-${tag}@example.com`,
    name: "E2E Delete",
  };
  const doomedToken = await mintToken(request, doomed);
  const meQuery = "{ me { id email } }";

  const before = await graphql<{ me: { id: string; email: string } }>(
    request,
    doomedToken,
    meQuery
  );
  expect(before.me.email).toBe(doomed.email);
  const deletedId = before.me.id;

  const del = await graphql<{ deleteMyAccount: boolean }>(
    request,
    doomedToken,
    "mutation { deleteMyAccount }"
  );
  expect(del.deleteMyAccount).toBe(true);

  // The same provider token re-provisions a fresh empty account — the
  // deleted row (and everything it owned) stays gone.
  const after = await graphql<{ me: { id: string; email: string } }>(
    request,
    doomedToken,
    meQuery
  );
  expect(after.me.id).not.toBe(deletedId);
  expect(after.me.email).toBe(doomed.email);

  // Exactly one live row carries the email — the deleted one isn't
  // lingering alongside its replacement.
  const adminToken = await mintToken(request, PRIMARY_USER);
  const listed = await graphql<{
    users: { items: { id: string; email: string }[]; pageInfo: { totalCount: number } };
  }>(
    request,
    adminToken,
    "query { users(page: 1, pageSize: 200) { items { id email } pageInfo { totalCount } } }"
  );
  const matches = listed.users.items.filter((u) => u.email === doomed.email);
  expect(matches).toHaveLength(1);
  expect(matches[0].id).toBe(after.me.id);
});
