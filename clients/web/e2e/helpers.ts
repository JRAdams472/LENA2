import { APIRequestContext, expect } from "@playwright/test";

const ISSUER_URL = process.env.E2E_ISSUER_URL ?? "http://localhost:8085";
const API_URL = `${process.env.E2E_BASE_URL ?? "http://localhost"}/graphql`;

export interface TestIdentity {
  sub: string;
  email: string;
  name: string;
}

export const PRIMARY_USER: TestIdentity = {
  sub: "e2e-user-1",
  email: "e2e@example.com",
  name: "E2E User",
};

export const SECOND_USER: TestIdentity = {
  sub: "e2e-user-2",
  email: "e2e-other@example.com",
  name: "E2E Other",
};

/** Mints a signed ID token from the local test issuer service. */
export async function mintToken(
  request: APIRequestContext,
  identity: TestIdentity = PRIMARY_USER
): Promise<string> {
  const res = await request.get(
    `${ISSUER_URL}/token?sub=${identity.sub}&email=${encodeURIComponent(
      identity.email
    )}&name=${encodeURIComponent(identity.name)}`
  );
  expect(res.ok()).toBeTruthy();
  const body = (await res.json()) as { id_token: string };
  return body.id_token;
}

/** Executes a GraphQL operation against the BFF and returns `data`. */
export async function graphql<T = Record<string, unknown>>(
  request: APIRequestContext,
  token: string | null,
  query: string,
  variables: Record<string, unknown> = {}
): Promise<T> {
  const res = await request.post(API_URL, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    data: { query, variables },
  });
  const payload = (await res.json()) as {
    data?: T;
    errors?: { message: string }[];
  };
  if (payload.errors?.length) {
    throw new Error(
      payload.errors.map((e) => e.message).join("; ")
    );
  }
  if (payload.data == null) {
    throw new Error(`GraphQL response had no data (HTTP ${res.status()})`);
  }
  return payload.data;
}

let counter = 0;
/** Returns a value unique to this test run, so reruns don't collide. */
export function unique(prefix: string): string {
  return `${prefix} ${Date.now().toString(36)}-${counter++}`;
}

/** Returns a unique two-letter code (e.g. for country iso_code, which is unique). */
export function uniqueCode(): string {
  const a = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";
  return `Z${a[Math.floor(Math.random() * a.length)]}`;
}
