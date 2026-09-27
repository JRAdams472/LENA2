import { test, expect } from "@playwright/test";
import { mintToken, unique, PRIMARY_USER, graphql } from "./helpers";

const API_URL = `${process.env.E2E_BASE_URL ?? "http://localhost"}/graphql`;

const CREATE_PLAN = /* GraphQL */ `
  mutation CreatePlan($name: String!, $weekStart: String!) {
    createMealPlan(input: { name: $name, weekStartDate: $weekStart }) {
      id
      name
    }
  }
`;

// Idempotency: a retried mutation with the same key must replay the
// stored response instead of executing a second time.
test.describe("idempotency", () => {
  test("same key + same payload replays the stored response", async ({
    request,
  }) => {
    const token = await mintToken(request, PRIMARY_USER);
    const key = `e2e-${Date.now()}`;
    const variables = {
      name: unique("Idem Plan"),
      weekStart: "2026-09-28",
    };
    const headers = {
      Authorization: `Bearer ${token}`,
      "Idempotency-Key": key,
    };

    const first = await request.post(API_URL, {
      headers,
      data: { query: CREATE_PLAN, variables },
    });
    expect(first.ok()).toBeTruthy();
    const firstBody = await first.json();
    const planId = firstBody.data.createMealPlan.id;

    const second = await request.post(API_URL, {
      headers,
      data: { query: CREATE_PLAN, variables },
    });
    expect(second.ok()).toBeTruthy();
    expect(second.headers()["idempotency-replayed"]).toBe("true");
    const secondBody = await second.json();

    // The replayed response is identical and no second plan was created.
    expect(secondBody).toEqual(firstBody);
    expect(secondBody.data.createMealPlan.id).toBe(planId);
  });

  test("same key + different payload is rejected", async ({ request }) => {
    const token = await mintToken(request, PRIMARY_USER);
    const key = `e2e-${Date.now()}-reuse`;
    const headers = {
      Authorization: `Bearer ${token}`,
      "Idempotency-Key": key,
    };

    const first = await request.post(API_URL, {
      headers,
      data: {
        query: CREATE_PLAN,
        variables: { name: unique("Reuse A"), weekStart: "2026-09-28" },
      },
    });
    expect(first.ok()).toBeTruthy();

    const second = await request.post(API_URL, {
      headers,
      data: {
        query: CREATE_PLAN,
        variables: { name: unique("Reuse B"), weekStart: "2026-09-28" },
      },
    });
    const body = await second.json();
    const codes = (body.errors ?? []).map(
      (e: { extensions?: { code?: string } }) => e.extensions?.code
    );
    expect(codes).toContain("IDEMPOTENCY_KEY_REUSED");
  });

  test("queries are never deduplicated", async ({ request }) => {
    const token = await mintToken(request, PRIMARY_USER);
    const key = `e2e-${Date.now()}-query`;
    const headers = {
      Authorization: `Bearer ${token}`,
      "Idempotency-Key": key,
    };
    const query = "{ mealPlans(pageSize: 1) { items { id } } }";

    const first = await request.post(API_URL, { headers, data: { query } });
    expect(first.ok()).toBeTruthy();
    const second = await request.post(API_URL, { headers, data: { query } });
    expect(second.ok()).toBeTruthy();
    // Same key on a query must not produce a replay marker.
    expect(second.headers()["idempotency-replayed"]).toBeUndefined();
  });

  test("identical retries inside the fallback window dedupe without a key", async ({
    request,
  }) => {
    const token = await mintToken(request, PRIMARY_USER);
    const variables = {
      name: unique("Auto Dedup"),
      weekStart: "2026-09-28",
    };

    // Byte-identical bodies with no Idempotency-Key: the payload-hash
    // fallback should replay the second request.
    const data = { query: CREATE_PLAN, variables };
    const first = await request.post(API_URL, {
      headers: { Authorization: `Bearer ${token}` },
      data,
    });
    expect(first.ok()).toBeTruthy();
    const second = await request.post(API_URL, {
      headers: { Authorization: `Bearer ${token}` },
      data,
    });
    expect(second.ok()).toBeTruthy();
    expect(second.headers()["idempotency-replayed"]).toBe("true");

    // Only one plan with this name exists.
    const plans = await graphql<{
      mealPlans: { items: { id: string; name: string }[] };
    }>(request, token, "{ mealPlans(pageSize: 50) { items { id name } } }");
    const matches = plans.mealPlans.items.filter(
      (p) => p.name === variables.name
    );
    expect(matches).toHaveLength(1);
  });
});
