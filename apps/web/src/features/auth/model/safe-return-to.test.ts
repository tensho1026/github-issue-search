import { describe, expect, it } from "vitest";

import { safeReturnTo } from "./safe-return-to";

const origin = "https://issuescout.example";

describe("safeReturnTo", () => {
  it.each([
    ["/", "/"],
    ["/search?username=octocat&auth=success", "/search?username=octocat"],
    ["/repositories?language=Go", "/repositories?language=Go"],
    ["/profiles/octocat", "/profiles/octocat"],
    [
      "/issues/octocat/repository/42?from=%2Fsearch",
      "/issues/octocat/repository/42?from=%2Fsearch",
    ],
    ["/workspace?tab=saved", "/workspace?tab=saved"],
  ])("preserves supported local routes: %s", (candidate, expected) => {
    expect(safeReturnTo(candidate, origin)).toBe(expected);
  });

  it.each([
    "https://evil.example/search",
    "//evil.example/search",
    "/\\evil",
    "/search#token",
    "/admin",
    "/issues/owner/repository/0",
    `/search?value=${"x".repeat(2048)}`,
  ])("falls back for an unsafe path: %s", (candidate) => {
    expect(safeReturnTo(candidate, origin)).toBe("/");
  });
});
