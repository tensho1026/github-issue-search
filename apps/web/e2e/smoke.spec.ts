import { expect, test } from "@playwright/test";

import {
  issueDetailFixture,
  issueSearchFixture,
} from "../src/test/issue-fixtures";
import gitHubUserFixture from "../../../packages/contracts/fixtures/github-user.success.json" with { type: "json" };
import profileAnalysisFixture from "../../../packages/contracts/fixtures/profile-analysis.success.json" with { type: "json" };

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

test("completes profile, search, and detail through the built API", async ({
  page,
}) => {
  await page.goto("/");

  await page.getByRole("textbox", { name: "GitHub username" }).fill("octocat");
  await page.getByRole("button", { name: "Analyze profile" }).click();

  await expect(page).toHaveURL("/profiles/octocat");
  await expect(
    page.getByRole("heading", { level: 1, name: "The Octocat" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Find matching issues" }).click();
  await expect(page).toHaveURL(/\/search\?username=octocat/);

  await page.getByRole("button", { name: "Find ranked issues" }).click();
  await expect(
    page.getByRole("heading", {
      name: "Improve keyboard navigation in the command palette",
    }),
  ).toBeVisible();
  await page.getByRole("link", { name: "View recommendation details" }).click();

  const detailHeading = page.getByRole("heading", {
    level: 1,
    name: "Improve keyboard navigation in the command palette",
  });
  await expect(detailHeading).toBeVisible();
  await expect(detailHeading).toBeFocused();
  await expect(
    page.getByRole("heading", { name: "Contributor readiness" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open original GitHub issue" }),
  ).toHaveAttribute(
    "href",
    "https://github.com/octocat/typed-service/issues/42",
  );
});

test("renders built API not-found and rate-limit states", async ({ page }) => {
  await page.goto("/profiles/missing-user");
  await expect(
    page.getByRole("heading", { level: 1, name: "Profile not found" }),
  ).toBeVisible();

  await page.goto("/profiles/rate-limited");
  await expect(
    page.getByRole("heading", { level: 1, name: "GitHub needs a breather" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Retry analysis" }),
  ).toBeVisible();
});

test("renders an explicit empty search from the built API", async ({
  page,
}) => {
  await page.goto("/search?username=no-results&search=1");

  await expect(
    page.getByRole("heading", { name: "No eligible issues found" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Broaden the filters" }),
  ).toBeVisible();
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

test("submits shareable issue filters and restores server pagination history", async ({
  page,
}) => {
  const requestBodies: unknown[] = [];
  await page.route("**/api/issues/search**", async (route) => {
    const requestURL = new URL(route.request().url());
    const requestedPage = Number(requestURL.searchParams.get("page") ?? "1");
    requestBodies.push(route.request().postDataJSON());
    await route.fulfill({
      body: JSON.stringify({
        ...issueSearchFixture,
        data: {
          ...issueSearchFixture.data,
          pagination: {
            ...issueSearchFixture.data.pagination,
            hasNext: requestedPage < 2,
            page: requestedPage,
          },
        },
      }),
      contentType: "application/json",
      status: 200,
    });
  });

  await page.goto(
    "/search?username=octocat&language=TypeScript&framework=React",
  );
  await expect(page.getByText("Shape a realistic search")).toBeVisible();
  await expect(page.getByRole("button", { name: "Languages" })).toContainText(
    "TypeScript",
  );

  await page.getByRole("combobox", { name: "Available time" }).click();
  await page.getByRole("option", { name: "Up to half a day" }).click();
  await page.getByRole("slider", { name: "Maximum difficulty" }).fill("4");
  await page.getByRole("checkbox", { name: /Include documentation/ }).check();
  await page.getByRole("button", { name: "Find ranked issues" }).click();

  await expect(page).toHaveURL(/search=1/);
  await expect(
    page.getByRole("heading", {
      name: "Improve keyboard navigation in the command palette",
    }),
  ).toBeVisible();
  expect(requestBodies[0]).toMatchObject({
    frameworks: ["React"],
    includeDocumentation: true,
    languages: ["TypeScript"],
    maximumDifficulty: 4,
    maximumEffort: "half_day",
    username: "octocat",
  });

  await page.getByRole("button", { name: "Go to page 2" }).click();
  await expect(page).toHaveURL(/page=2/);
  await expect(page.getByText("Page 2 of 2")).toBeVisible();

  await page.goBack();
  await expect(page).toHaveURL(/page=1/);
  await expect(page.getByText("Page 1 of 2")).toBeVisible();
  await page.goForward();
  await expect(page).toHaveURL(/page=2/);
  await expect(page.getByText("Page 2 of 2")).toBeVisible();
});

test("keeps search popovers usable on a mobile viewport", async ({ page }) => {
  await page.setViewportSize({ height: 844, width: 390 });
  await page.goto("/search?username=octocat");

  const languages = page.getByRole("button", { name: "Languages" });
  await languages.click();
  await expect(
    page.getByRole("searchbox", { name: "Search languages" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(languages).toBeFocused();
});

test("opens a safe issue detail and restores the exact ranked search", async ({
  page,
}) => {
  const detail = structuredClone(issueDetailFixture);
  detail.data.issue.body =
    "<script>globalThis.compromised=true</script>\n[unsafe](javascript:alert(1))";
  let detailRequest: URL | undefined;
  await page.route("**/api/issues/search**", async (route) => {
    await route.fulfill({
      body: JSON.stringify({
        ...issueSearchFixture,
        data: {
          ...issueSearchFixture.data,
          pagination: {
            ...issueSearchFixture.data.pagination,
            hasNext: false,
            page: 2,
          },
        },
      }),
      contentType: "application/json",
      status: 200,
    });
  });
  await page.route(
    "**/api/issues/octocat/typed-service/42**",
    async (route) => {
      detailRequest = new URL(route.request().url());
      await route.fulfill({
        body: JSON.stringify(detail),
        contentType: "application/json",
        status: 200,
      });
    },
  );
  await page.setViewportSize({ height: 844, width: 390 });
  const searchURL =
    "/search?username=octocat&language=TypeScript&framework=React&page=2&search=1";
  await page.goto(searchURL);

  await page.getByRole("link", { name: "View recommendation details" }).click();

  const title = page.getByRole("heading", {
    level: 1,
    name: "Improve keyboard navigation in the command palette",
  });
  await expect(title).toBeVisible();
  await expect(title).toBeFocused();
  await expect(
    page.getByRole("heading", { name: "Contributor readiness" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Maintainer activity" }),
  ).toBeVisible();
  await expect(
    page.getByText(/<script>globalThis\.compromised=true<\/script>/),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: "unsafe" })).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Open original GitHub issue" }),
  ).toHaveAttribute(
    "href",
    "https://github.com/octocat/typed-service/issues/42",
  );
  expect(detailRequest?.searchParams.getAll("skills")).toEqual([
    "TypeScript",
    "React",
  ]);
  const overflow =
    await page.evaluate(`Array.from(document.querySelectorAll("*"))
    .filter((element) => {
      const bounds = element.getBoundingClientRect();
      return bounds.left < -1 || bounds.right > window.innerWidth + 1;
    })
    .map((element) => ({
      className: element.className,
      right: Math.round(element.getBoundingClientRect().right),
      tagName: element.tagName,
      text: element.textContent?.slice(0, 80),
    }))`);
  expect(overflow).toEqual([]);

  await page.getByRole("link", { name: "Back to search results" }).click();
  await expect(page).toHaveURL(searchURL);
  await expect(page.getByText("Page 2 of 2")).toBeVisible();
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
