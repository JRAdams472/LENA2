import { defineConfig, devices } from "@playwright/test";

export const AUTH_FILE = "e2e/.auth/user.json";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  // CI emits a JUnit report alongside the HTML report so per-test durations
  // are identifiable from the uploaded artifact without downloading traces.
  reporter: process.env.CI
    ? [["html"], ["junit", { outputFile: "test-results/junit.xml" }]]
    : "html",
  use: {
    // The docker compose stack (Caddy) fronts both the web app and /graphql.
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost",
    trace: "on-first-retry",
  },
  projects: [
    { name: "setup", testMatch: /auth\.setup\.ts/ },
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"], storageState: AUTH_FILE },
      testIgnore: /responsive\.spec\.ts/,
      dependencies: ["setup"],
    },
    {
      // LEN-30: the ~375px fallback experience — runs only the responsive
      // spec since it re-verifies journeys the desktop specs already own.
      name: "mobile",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 375, height: 812 },
        storageState: AUTH_FILE,
      },
      testMatch: /responsive\.spec\.ts/,
      dependencies: ["setup"],
    },
  ],
});
