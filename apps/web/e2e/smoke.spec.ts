import { expect, test } from "@playwright/test";

const apiBaseURL = "http://127.0.0.1:18080";

test("serves the production application shell with keyboard navigation", async ({
  page,
}) => {
  await page.goto("/");

  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Find the issue you can finish.",
    }),
  ).toBeVisible();
  await expect(page.getByText("Recommendation model")).toBeVisible();

  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "IssueScout" })).toBeFocused();
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
