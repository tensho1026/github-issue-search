import { describe, expect, it } from "vitest";

import {
  createDefaultSearchFilters,
  decodeSearchParams,
  encodeSearchParams,
  toIssueSearchRequest,
  validateSearchFilters,
} from "./search-filters";

describe("issue search filter model", () => {
  it("round-trips every filter and pagination value through the URL", () => {
    const filters = {
      ...createDefaultSearchFilters("Octo-Cat"),
      excludeArchived: false,
      frameworks: ["React", "Gin"],
      includeDocumentation: true,
      includeEnglish: false,
      labels: ["bug", "accessibility"],
      languages: ["TypeScript", "Go"],
      maximumDifficulty: 4,
      maximumEffort: "one_day" as const,
      minimumStars: 25,
      page: 2,
      perPage: 10,
      updatedWithinDays: 90,
    };

    const parameters = encodeSearchParams(filters);
    const decoded = decodeSearchParams(parameters);

    expect(decoded).toEqual({
      errors: {},
      filters,
      shouldSearch: true,
      valid: true,
    });
    expect(parameters.getAll("language")).toEqual(["TypeScript", "Go"]);
    expect(parameters.get("search")).toBe("1");
  });

  it("uses backend-aligned defaults before the first search", () => {
    expect(decodeSearchParams(new URLSearchParams())).toEqual({
      errors: {
        username: "Enter a GitHub username to continue.",
      },
      filters: createDefaultSearchFilters(),
      shouldSearch: false,
      valid: false,
    });
  });

  it("rejects malformed or duplicated shared URL parameters", () => {
    const parameters = encodeSearchParams(
      createDefaultSearchFilters("octocat"),
    );
    parameters.append("page", "2");
    parameters.set("minimumStars", "-1");
    parameters.set("includeEnglish", "yes");

    const decoded = decodeSearchParams(parameters);

    expect(decoded.valid).toBe(false);
    expect(decoded.shouldSearch).toBe(true);
    expect(decoded.errors.form).toMatch(/shared search URL is invalid/i);
    expect(decoded.errors.form).toMatch(/page.*provided once/i);
    expect(decoded.errors.form).toMatch(/minimumStars.*integer/i);
    expect(decoded.errors.form).toMatch(/includeEnglish/i);
  });

  it("rejects unsafe and duplicate filter values before an API request", () => {
    const errors = validateSearchFilters({
      ...createDefaultSearchFilters("octocat"),
      frameworks: ["React", " react "],
      languages: ['Go" language'],
    });

    expect(errors.frameworks).toMatch(/unique/i);
    expect(errors.languages).toMatch(/cannot contain/i);
  });

  it("maps validated UI state to the documented request body", () => {
    const request = toIssueSearchRequest({
      ...createDefaultSearchFilters(" OctoCat "),
      frameworks: [" React "],
      maximumEffort: "half_day",
    });

    expect(request).toEqual({
      excludeArchived: true,
      frameworks: ["React"],
      includeDocumentation: false,
      includeEnglish: true,
      labels: ["good first issue", "help wanted"],
      languages: [],
      maximumDifficulty: 3,
      maximumEffort: "half_day",
      minimumStars: 10,
      updatedWithinDays: 180,
      username: "OctoCat",
    });
  });

  it("omits maximumEffort when every effort band is allowed", () => {
    expect(
      toIssueSearchRequest(createDefaultSearchFilters("octocat")),
    ).not.toHaveProperty("maximumEffort");
  });

  it("refuses to serialize invalid state", () => {
    expect(() =>
      encodeSearchParams({
        ...createDefaultSearchFilters("invalid--user"),
        perPage: 100,
      }),
    ).toThrow(/invalid issue search filters/i);
  });
});
