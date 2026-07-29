import { defineConfig, devices } from "@playwright/test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));
const webBaseURL = "http://127.0.0.1:4173";
const apiBaseURL = "http://127.0.0.1:18080";

export default defineConfig({
  expect: {
    timeout: 5_000,
  },
  forbidOnly: Boolean(process.env.CI),
  fullyParallel: true,
  outputDir: "../../test-results/e2e",
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
      },
    },
  ],
  reporter: [
    ["list"],
    [
      "html",
      {
        open: "never",
        outputFolder: "../../playwright-report",
      },
    ],
  ],
  retries: process.env.CI ? 2 : 0,
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: webBaseURL,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    video: "retain-on-failure",
  },
  webServer: [
    {
      command:
        "pnpm --filter @issuescout/web preview --host 127.0.0.1 --port 4173",
      cwd: repositoryRoot,
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
      url: webBaseURL,
    },
    {
      command: "./bin/issuescout-api",
      cwd: repositoryRoot,
      env: {
        ALLOWED_ORIGINS: webBaseURL,
        PORT: "18080",
      },
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
      url: `${apiBaseURL}/api/health`,
    },
  ],
});
