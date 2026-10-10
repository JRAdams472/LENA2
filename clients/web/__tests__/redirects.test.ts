/**
 * @jest-environment node
 */
import nextConfig from "../next.config";

describe("legacy member-path redirects", () => {
  const redirects = () =>
    nextConfig.redirects ? nextConfig.redirects() : Promise.resolve([]);

  it("permanently redirects every legacy top-level member path to /member", async () => {
    const rules = await redirects();
    const expected = [
      "assistant",
      "events",
      "grocery-lists",
      "household",
      "inventory",
      "items",
      "meal-plans",
      "notifications",
      "profile",
      "recipes",
      "users",
      "wine",
    ];
    for (const path of expected) {
      const rule = rules.find(
        (r) => r.source === `/${path}/:path*` || r.source === `/${path}`
      );
      if (!rule) throw new Error(`missing redirect for /${path}`);
      expect(rule!.destination).toMatch(/^\/member\//);
      expect(rule!.permanent).toBe(true);
    }
  });

  it("does not redirect the public surface", async () => {
    const rules = await redirects();
    for (const rule of rules) {
      expect(rule.source).not.toBe("/");
      expect(rule.source).not.toMatch(/^\/docs/);
      expect(rule.source).not.toMatch(/^\/auth/);
      expect(rule.source).not.toBe("/login");
    }
  });
});
