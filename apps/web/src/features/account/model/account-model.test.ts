import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../../../shared/api/client";
import type { SavedSearch } from "../../../shared/api/generated";
import { accountErrorPresentation } from "./account-errors";
import { applyPreferences } from "./preferences";
import { savedSearchRoute } from "./saved-search-route";

afterEach(() => {
  document.documentElement.removeAttribute("data-reduced-motion");
  document.documentElement.removeAttribute("data-theme");
  vi.unstubAllGlobals();
});

describe("accountErrorPresentation", () => {
  it.each([
    [401, "Session expired"],
    [409, "Newer account data exists"],
    [503, "Account storage unavailable"],
    [400, "Check the submitted values"],
    [500, "Account request failed"],
  ])("maps status %d to a safe recovery state", (status, title) => {
    const error = new ApiError({
      code: "TEST",
      message: "private upstream detail",
      status,
    });
    expect(accountErrorPresentation(error).title).toBe(title);
    expect(accountErrorPresentation(error).description).not.toContain(
      "private upstream detail",
    );
  });

  it("maps non-API failures without exposing details", () => {
    expect(accountErrorPresentation(new Error("secret")).title).toBe(
      "Account request failed",
    );
  });
});

describe("applyPreferences", () => {
  it("applies explicit preferences without browser storage", () => {
    applyPreferences({ reducedMotion: "reduce", theme: "dark" });
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.documentElement.dataset.reducedMotion).toBe("reduce");
  });

  it("resolves system preferences through media queries", () => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn((query: string) => ({ matches: query.includes("dark") })),
    );
    applyPreferences({ reducedMotion: "system", theme: "system" });
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.documentElement.dataset.reducedMotion).toBe(
      "no-preference",
    );
  });
});

describe("savedSearchRoute", () => {
  const base = {
    createdAt: "2026-08-01T00:00:00Z",
    id: "saved",
    name: "Saved",
    updatedAt: "2026-08-01T00:00:00Z",
    version: 1,
  };

  it("reconstructs an executable issue-search URL", () => {
    const search: SavedSearch = {
      ...base,
      filters: {
        includeEnglish: false,
        labels: ["documentation"],
        maximumEffort: "two_hours",
        username: "octocat",
      },
      searchType: "issue",
    };
    const route = new URL(savedSearchRoute(search), "https://example.test");
    expect(route.pathname).toBe("/search");
    expect(route.searchParams.get("username")).toBe("octocat");
    expect(route.searchParams.get("maximumEffort")).toBe("two_hours");
    expect(route.searchParams.get("search")).toBe("1");
  });

  it.each([
    [true, "yes"],
    [false, "no"],
    [undefined, "any"],
  ] as const)(
    "reconstructs repository filters with Japanese README %s",
    (hasJapaneseReadme, expected) => {
      const search: SavedSearch = {
        ...base,
        filters: { hasJapaneseReadme, minimumStars: 50 },
        searchType: "repository",
      };
      const route = new URL(savedSearchRoute(search), "https://example.test");
      expect(route.pathname).toBe("/repositories");
      expect(route.searchParams.get("japaneseReadme")).toBe(expected);
      expect(route.searchParams.get("minimumStars")).toBe("50");
    },
  );
});
