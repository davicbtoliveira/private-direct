import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  retries: 0,
  reporter: "list",
  use: {
    baseURL: process.env.APP_URL ?? "http://127.0.0.1:8080",
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },
    {
      name: "webkit",
      use: { ...devices["Desktop Safari"] },
    },
  ],
  webServer: {
    command: "go run ./cmd/privatedirect",
    cwd: "../",
    url: "http://127.0.0.1:8080/health",
    reuseExistingServer: !process.env.CI,
    env: {
      PRIVATE_DIRECT_ADDR: "127.0.0.1:8080",
      PRIVATE_DIRECT_DB: ":memory:",
      PRIVATE_DIRECT_OPERATOR_TOKEN: "operator-secret",
      PRIVATE_DIRECT_JWT_SECRET: "test-jwt-secret",
      PRIVATE_DIRECT_STUN_URLS: "stun:test.example",
    },
    timeout: 60_000,
  },
});
