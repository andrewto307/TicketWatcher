import { defineConfig, devices } from "@playwright/test";

// These run against the real Go binary serving the embedded SPA, with a real
// database — that is the point of using a browser rather than jsdom. Only
// /api/search is stubbed (see e2e/smoke.spec.ts): it is the one endpoint backed
// by an external service, so live results would make assertions flaky and every
// run would spend real Ticketmaster budget.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? "line" : [["list"]],
  timeout: 30_000,
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:8080",
    trace: "retain-on-failure",
  },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"] } },
    { name: "mobile", use: { ...devices["Pixel 5"] } },
  ],
  // Reuses an already-running server when there is one, so a dev loop does not
  // pay the start-up cost on every run.
  webServer: {
    command: "go run ./cmd/api",
    cwd: "..",
    url: "http://localhost:8080/healthz",
    reuseExistingServer: true,
    timeout: 120_000,
  },
});
