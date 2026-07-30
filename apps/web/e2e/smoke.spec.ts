import { expect, test } from "@playwright/test";

import {
  gitHubUserFixture,
  profileAnalysisFixture,
} from "../src/test/profile-fixtures";

const apiBaseURL = "http://127.0.0.1:18080";

test("serves the production application shell with keyboard navigation", async ({
  page,
}) => {
  await page.goto("/");

  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Your next contribution, decoded.",
    }),
  ).toBeVisible();
  await expect(page.getByText("Recommendation anatomy")).toBeVisible();

  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Skip to content" }),
  ).toBeFocused();
});

test("analyzes a valid username through the production profile route", async ({
  page,
}) => {
  await page.route("**/api/github/users/octocat**", async (route) => {
    const payload = route.request().url().endsWith("/profile-analysis")
      ? profileAnalysisFixture
      : gitHubUserFixture;
    await route.fulfill({
      body: JSON.stringify(payload),
      contentType: "application/json",
      status: 200,
    });
  });
  await page.goto("/");

  await page.getByRole("textbox", { name: "GitHub username" }).fill("octocat");
  await page.getByRole("button", { name: "Analyze profile" }).click();

  await expect(page).toHaveURL("/profiles/octocat");
  await expect(
    page.getByRole("heading", { level: 1, name: "The Octocat" }),
  ).toBeVisible();
  await expect(
    page.getByRole("progressbar", { name: "TypeScript 65%" }),
  ).toBeVisible();
  await expect(page.getByText("typed-service")).toBeVisible();

  const languageOrder = page.getByRole("combobox", { name: "Sort languages" });
  await languageOrder.press("ArrowDown");
  await page.getByRole("option", { name: "A–Z" }).click();
  await expect(languageOrder).toContainText("A–Z");
});

test("rejects malformed usernames without making an API request", async ({
  page,
}) => {
  let apiRequests = 0;
  page.on("request", (request) => {
    if (request.url().includes("/api/github/users/")) {
      apiRequests += 1;
    }
  });
  await page.goto("/");

  await page
    .getByRole("textbox", { name: "GitHub username" })
    .fill("invalid--user");
  await page.getByRole("button", { name: "Analyze profile" }).click();

  await expect(page.getByRole("alert")).toContainText(
    "letters, numbers, or single hyphens",
  );
  await expect(page).toHaveURL("/");
  expect(apiRequests).toBe(0);
});

test("keeps mobile navigation keyboard accessible", async ({ page }) => {
  await page.setViewportSize({ height: 844, width: 390 });
  await page.goto("/");

  const trigger = page.getByRole("button", { name: "Open navigation" });
  await trigger.click();

  await expect(
    page.getByRole("dialog", { name: "Navigate IssueScout" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
});

test("serves the built API with the shared response envelope", async ({
  request,
}) => {
  const requestID = "e2e_health_request";
  const response = await request.get(`${apiBaseURL}/api/health`, {
    headers: {
      "X-Request-ID": requestID,
    },
  });

  expect(response.status()).toBe(200);
  expect(response.headers()["x-content-type-options"]).toBe("nosniff");
  await expect(response.json()).resolves.toMatchObject({
    data: {
      status: "ok",
    },
    meta: {
      requestId: requestID,
    },
  });
});
