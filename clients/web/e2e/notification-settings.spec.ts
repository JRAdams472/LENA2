import { test, expect, type APIRequestContext } from "@playwright/test";
import { graphql, mintToken } from "./helpers";

// Dedicated users keep this spec's unread counts and memberships isolated
// from household.spec.ts and notifications.spec.ts running in parallel.
const RECIPIENT = {
  sub: "e2e-notify-owner",
  email: "e2e-notify-owner@example.com",
  name: "E2E Notify Owner",
};
const JOINER = {
  sub: "e2e-notify-joiner",
  email: "e2e-notify-joiner@example.com",
  name: "E2E Notify Joiner",
};

// notifications.spec.ts covers expiry reminders and plain category
// persistence; this spec drives a real household event through the
// suppression gate after the recipient turns the category off and mutes it.

// The BFF dedups byte-identical mutation bodies for 30s (even across
// runs), so invite/accept/leave calls get a unique operation name.
let opSeq = 0;
const runTag = Date.now().toString(36);
const op = (name: string) => `${name}_${runTag}_${++opSeq}`;

const unread = async (request: APIRequestContext, token: string) =>
  (
    await graphql<{ unreadNotificationCount: number }>(
      request,
      token,
      `{ unreadNotificationCount }`
    )
  ).unreadNotificationCount;

async function activeHouseholdId(request: APIRequestContext, token: string) {
  const res = await graphql<{ myHouseholds: { id: string; isActive: boolean }[] }>(
    request,
    token,
    `{ myHouseholds { id isActive } }`
  );
  return res.myHouseholds.find((h) => h.isActive)?.id ?? null;
}

async function leaveIfMember(
  request: APIRequestContext,
  token: string,
  householdId: string
) {
  const res = await graphql<{ myHouseholds: { id: string }[] }>(
    request,
    token,
    `{ myHouseholds { id } }`
  );
  if (res.myHouseholds.some((h) => h.id === householdId)) {
    await graphql(request, token, `mutation ${op("Leave")}($id: ID) { leaveHousehold(householdId: $id) }`, {
      id: householdId,
    });
  }
}

/** The owner invites the joiner, who joins without merging. */
async function joinPrimaryHousehold(
  request: APIRequestContext,
  primaryToken: string,
  secondToken: string,
  secondId: string
) {
  await graphql(
    request,
    primaryToken,
    `mutation ${op("Invite")}($id: ID!) { inviteHouseholdMember(userId: $id) { id } }`,
    { id: secondId }
  );
  const res = await graphql<{
    householdInvites: { id: string; status: string; toUser: { id: string } }[];
  }>(request, secondToken, `{ householdInvites { id status toUser { id } } }`);
  const invite = res.householdInvites.find(
    (i) => i.toUser.id === secondId && /pending/i.test(i.status)
  );
  expect(invite, "pending invite for the second user").toBeTruthy();
  await graphql(
    request,
    secondToken,
    `mutation ${op("Accept")}($id: ID!) { acceptHouseholdInvite(inviteId: $id) { id } }`,
    { id: invite!.id }
  );
}

test.describe("notification settings", () => {
  test("a disabled, muted household category stops bell unreads", async ({
    browser,
    request,
    baseURL,
  }) => {
    test.setTimeout(60_000);
    const primaryToken = await mintToken(request, RECIPIENT);
    const secondToken = await mintToken(request, JOINER);
    const secondId = (
      await graphql<{ updateMyProfile: { id: string } }>(
        request,
        secondToken,
        `mutation { updateMyProfile(input: { isSearchable: true }) { id } }`
      )
    ).updateMyProfile.id;
    let target = await activeHouseholdId(request, primaryToken);
    if (!target) {
      target = (
        await graphql<{ createHousehold: { id: string } }>(
          request,
          primaryToken,
          `mutation { createHousehold(name: "E2E Notify Home") { id } }`
        )
      ).createHousehold.id;
    }
    await leaveIfMember(request, secondToken, target);

    const ctx = await browser.newContext({ baseURL });
    await ctx.addInitScript(
      (t) => window.localStorage.setItem("lena_id_token", t),
      primaryToken
    );
    const page = await ctx.newPage();
    const row = page.getByRole("listitem").filter({ hasText: "Household activity" });
    const enabled = row.getByRole("switch", { name: "Enable Household activity" });

    try {
      // Control: with the category on, a member joining raises the count.
      let before = await unread(request, primaryToken);
      await joinPrimaryHousehold(request, primaryToken, secondToken, secondId);
      await expect.poll(() => unread(request, primaryToken)).toBeGreaterThan(before);
      await leaveIfMember(request, secondToken, target);

      await page.goto("/notifications");
      await expect(enabled).toBeChecked();
      await enabled.click();
      await expect(enabled).not.toBeChecked();
      await row.getByRole("button", { name: "Mute" }).click();
      await page.getByRole("menuitem", { name: "Mute for 1 hour" }).click();
      await expect(row.getByText(/^Muted until /)).toBeVisible();

      await page.reload();
      await expect(enabled).not.toBeChecked();
      await expect(row.getByText(/^Muted until /)).toBeVisible();
      const prefs = await graphql<{
        myNotificationPreferences: {
          category: string;
          enabled: boolean;
          mutedUntil: string | null;
        }[];
      }>(
        request,
        primaryToken,
        `{ myNotificationPreferences { category enabled mutedUntil } }`
      );
      const household = prefs.myNotificationPreferences.find((p) => p.category === "household");
      expect(household?.enabled).toBe(false);
      expect(Date.parse(household?.mutedUntil ?? "")).toBeGreaterThan(Date.now());

      before = await unread(request, primaryToken);
      await joinPrimaryHousehold(request, primaryToken, secondToken, secondId);
      expect(await activeHouseholdId(request, secondToken)).toBe(target);
      // Notifications are written in the accepting transaction, so a short
      // settle is enough to prove nothing arrived.
      await page.waitForTimeout(1_000);
      expect(await unread(request, primaryToken)).toBe(before);
    } finally {
      await leaveIfMember(request, secondToken, target).catch(() => undefined);
      await graphql(
        request,
        primaryToken,
        `mutation {
          setNotificationCategoryEnabled(category: "household", enabled: true)
          clearNotificationMute(category: "household")
        }`
      ).catch(() => undefined);
      await ctx.close();
    }
  });
});
