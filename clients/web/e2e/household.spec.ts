import { test, expect, type APIRequestContext } from "@playwright/test";
import { graphql, mintToken, unique, PRIMARY_USER } from "./helpers";

// A dedicated joiner: SECOND_USER must stay outside the primary household
// for auth.spec.ts's cross-user isolation checks running in parallel.
const JOINER = {
  sub: "e2e-household-joiner",
  email: "e2e-household-joiner@example.com",
  name: "E2E Joiner",
};

type Household = { id: string; name: string | null; isActive: boolean };

const householdLabel = (h: { name: string | null }) => h.name ?? "Unnamed household";

async function myHouseholds(
  request: APIRequestContext,
  token: string
): Promise<Household[]> {
  const res = await graphql<{ myHouseholds: Household[] }>(
    request,
    token,
    `{ myHouseholds { id name isActive } }`
  );
  return res.myHouseholds;
}

test.describe("household invites", () => {
  test("invite, accept with merge, see shared data, and switch households", async ({
    page,
    browser,
    request,
    baseURL,
  }) => {
    test.setTimeout(120_000);
    const primaryToken = await mintToken(request, PRIMARY_USER);
    const secondToken = await mintToken(request, JOINER);
    const spareName = unique("E2E Spare Home");
    const extraName = unique("E2E Extra Home");
    const planName = unique("E2E Shared Plan");

    // The second user must be discoverable and solely own a household so
    // accepting offers the merge prompt.
    const secondProfile = await graphql<{ updateMyProfile: { id: string } }>(
      request,
      secondToken,
      `mutation { updateMyProfile(input: { isSearchable: true }) { id } }`
    );
    const secondUserId = secondProfile.updateMyProfile.id;
    // The page offers the earliest sole-owned home as the merge source;
    // seed one only if the second user has none.
    const soleOwned = async () => {
      const res = await graphql<{
        myHouseholds: { id: string; name: string | null; myRole: string; members: unknown[] }[];
      }>(request, secondToken, `{ myHouseholds { id name myRole members { isMe } } }`);
      return res.myHouseholds.find((h) => h.members.length === 1 && h.myRole === "OWNER");
    };
    let source = await soleOwned();
    if (!source) {
      await graphql(
        request,
        secondToken,
        `mutation ($name: String) { createHousehold(name: $name) { id } }`,
        { name: spareName }
      );
      source = await soleOwned();
    }
    expect(source).toBeDefined();
    const spare = { createHousehold: { id: source!.id } };
    const sourceLabel = householdLabel(source!);

    const primaryHome = await graphql<{
      myHousehold: { id: string; name: string | null } | null;
    }>(request, primaryToken, `{ myHousehold { id name } }`);
    expect(primaryHome.myHousehold).not.toBeNull();
    const target = primaryHome.myHousehold!;
    if (!target.name) {
      // Give the target a label distinct from any unnamed fallback home.
      await graphql(
        request,
        primaryToken,
        `mutation { renameHousehold(name: "E2E Household") { id } }`
      );
      target.name = "E2E Household";
    }
    const targetLabel = householdLabel(target);

    // A prior aborted run may have left the second user in the target
    // household, which hides them from invite search.
    if ((await myHouseholds(request, secondToken)).some((h) => h.id === target.id)) {
      await graphql(
        request,
        secondToken,
        `mutation ($id: ID) { leaveHousehold(householdId: $id) }`,
        { id: target.id }
      );
    }

    // Household-scoped data the primary owns before the join.
    const plan = await graphql<{ createMealPlan: { id: string } }>(
      request,
      primaryToken,
      `mutation ($input: CreateMealPlanInput!) {
        createMealPlan(input: $input) { id }
      }`,
      { input: { name: planName, weekStartDate: "2030-01-07" } }
    );

    const second = await browser.newContext({ baseURL });
    await second.addInitScript(
      (t) => window.localStorage.setItem("lena_id_token", t),
      secondToken
    );
    const secondPage = await second.newPage();

    try {
      // Before joining, the second user can't see the primary's plan.
      await secondPage.goto("/meal-plans");
      await expect(secondPage.getByText(JOINER.email).first()).toBeVisible();
      await expect(secondPage.getByText(planName)).toHaveCount(0);

      // PRIMARY_USER invites through the search box.
      await page.goto("/household");
      await page.getByLabel("Name or email").fill(JOINER.name);
      await page.getByRole("button", { name: "Search", exact: true }).click();
      const hit = page
        .getByRole("listitem")
        .filter({ hasText: JOINER.name })
        .filter({ has: page.getByRole("button", { name: "Invite" }) });
      await hit.getByRole("button", { name: "Invite" }).click();
      await expect
        .poll(async () => {
          const res = await graphql<{
            householdInvites: { toUser: { id: string }; status: string }[];
          }>(request, primaryToken, `{ householdInvites { toUser { id } status } }`);
          return res.householdInvites.some(
            (i) => i.toUser.id === secondUserId && i.status === "PENDING"
          );
        })
        .toBe(true);

      // JOINER accepts in its own browser context; the sole-owned
      // spare home triggers the merge prompt.
      await secondPage.goto("/household");
      const invitations = secondPage
        .locator("div")
        .filter({ has: secondPage.getByText("Invitations", { exact: true }) })
        .filter({ hasText: PRIMARY_USER.name })
        .last();
      await invitations.getByRole("button", { name: "Accept" }).first().click();
      const merge = secondPage.getByRole("dialog", { name: "Join this household?" });
      await expect(merge).toBeVisible();
      await expect(merge).toContainText(`merge ${sourceLabel} into`);
      await merge.getByRole("button", { name: "Join and merge" }).click();
      await expect(merge).toBeHidden();

      await expect
        .poll(async () => {
          const hs = await myHouseholds(request, secondToken);
          return {
            active: hs.find((h) => h.isActive)?.id,
            spareGone: !hs.some((h) => h.id === spare.createHousehold.id),
          };
        })
        .toEqual({ active: target.id, spareGone: true });

      // Shared household data is now visible to the second user.
      await secondPage.goto("/meal-plans");
      await expect(secondPage.getByText(planName)).toBeVisible();

      // Switcher: create another home (it becomes active), then switch back.
      await secondPage.goto("/household");
      await secondPage.getByRole("button", { name: "New household" }).click();
      const create = secondPage.getByRole("dialog", { name: "New household" });
      await create.getByLabel("Household name").fill(extraName);
      await create.getByRole("button", { name: "Create" }).click();
      await expect(create).toBeHidden();
      await expect(
        secondPage.getByRole("button", { name: `Switch to ${targetLabel}` })
      ).toBeVisible();
      await secondPage
        .getByRole("button", { name: `Switch to ${targetLabel}` })
        .click();
      await expect(
        secondPage.getByRole("button", { name: `Switch to ${extraName}` })
      ).toBeVisible();
      const after = await myHouseholds(request, secondToken);
      expect(after.find((h) => h.isActive)?.id).toBe(target.id);
    } finally {
      await second.close();
      // Leave the extra home, then the primary's; the fallback leaves the
      // second user with a personal household again.
      for (const h of await myHouseholds(request, secondToken).catch(() => [])) {
        if (h.name === extraName || h.id === spare.createHousehold.id) {
          await graphql(
            request,
            secondToken,
            `mutation ($id: ID) { leaveHousehold(householdId: $id) }`,
            { id: h.id }
          ).catch(() => undefined);
        }
      }
      await graphql(
        request,
        secondToken,
        `mutation ($id: ID) { leaveHousehold(householdId: $id) }`,
        { id: target.id }
      ).catch((e) => console.warn("household cleanup: leave target failed", e));
      const invites = await graphql<{
        householdInvites: { id: string; toUser: { id: string }; status: string }[];
      }>(request, primaryToken, `{ householdInvites { id toUser { id } status } }`).catch(
        () => ({ householdInvites: [] })
      );
      for (const i of invites.householdInvites) {
        if (i.toUser.id === secondUserId && i.status === "PENDING") {
          await graphql(
            request,
            primaryToken,
            `mutation ($id: ID!) { cancelHouseholdInvite(inviteId: $id) { id } }`,
            { id: i.id }
          ).catch(() => undefined);
        }
      }
      await graphql(
        request,
        primaryToken,
        `mutation ($id: ID!) { deleteMealPlan(id: $id) }`,
        { id: plan.createMealPlan.id }
      ).catch(() => undefined);
    }
  });
});
