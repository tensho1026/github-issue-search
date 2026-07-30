import { describe, expect, it } from "vitest";

import {
  createDefaultRepositoryFilters,
  decodeRepositorySearchParams,
  encodeRepositorySearchParams,
  normalizeRepositoryFilters,
  toRepositoryDiscoveryRequest,
  validateRepositoryFilters,
  type RepositoryFilters,
} from "./repository-filters";

describe("repository discovery filters", () => {
  it("provides a valid conservative anonymous default", () => {
    const filters = createDefaultRepositoryFilters();
    expect(validateRepositoryFilters(filters)).toEqual({});
    expect(filters).toMatchObject({
      excludeArchived: true,
      forkPolicy: "exclude",
      maximumDifficulty: 3,
      minimumReadiness: 40,
      minimumStars: 10,
      page: 1,
      perPage: 20,
    });
  });

  it("round-trips every filter through a canonical shareable URL", () => {
    const filters: RepositoryFilters = {
      ...createDefaultRepositoryFilters(),
      categories: ["security", "library"],
      forkPolicy: "include",
      hasJapaneseReadme: "yes",
      languages: ["TypeScript", "Go"],
      licenses: ["MIT", "Apache-2.0"],
      maximumDifficulty: 4,
      maximumOpenIssues: 900,
      minimumForks: 5,
      minimumOpenIssues: 2,
      minimumReadiness: 65,
      minimumStars: 50,
      page: 2,
      perPage: 10,
      technologies: ["React", "Next.js"],
      updatedWithinDays: 90,
    };
    const parameters = encodeRepositorySearchParams(filters);
    const decoded = decodeRepositorySearchParams(parameters);

    expect(decoded).toEqual({
      errors: {},
      filters: normalizeRepositoryFilters(filters),
      shouldSearch: true,
      valid: true,
    });
    expect(parameters.toString()).not.toContain("result");
  });

  it("rejects unknown, duplicate, unsupported, and out-of-range URL values", () => {
    const parameters = encodeRepositorySearchParams(
      createDefaultRepositoryFilters(),
    );
    parameters.append("minimumStars", "11");
    parameters.set("license", "NOT-A-LICENSE");
    parameters.set("minimumReadiness", "101");
    parameters.set("payload", "do-not-store-results");

    const decoded = decodeRepositorySearchParams(parameters);
    expect(decoded.valid).toBe(false);
    expect(decoded.errors.form).toMatch(/minimumStars/i);
    expect(decoded.errors.form).toMatch(/payload/i);
    expect(decoded.errors.licenses).toMatch(/supported SPDX/i);
    expect(decoded.errors.minimumReadiness).toMatch(/0 and 100/i);
  });

  it("rejects inverted issue ranges and unsafe qualifier values", () => {
    const filters = {
      ...createDefaultRepositoryFilters(),
      languages: ['Type"Script'],
      maximumOpenIssues: 1,
      minimumOpenIssues: 2,
    };
    const errors = validateRepositoryFilters(filters);
    expect(errors.languages).toMatch(/quotes/i);
    expect(errors.maximumOpenIssues).toMatch(/at least/i);
  });

  it("maps the tri-state README condition without leaking pagination", () => {
    const anyRequest = toRepositoryDiscoveryRequest({
      ...createDefaultRepositoryFilters(),
      hasJapaneseReadme: "any",
      page: 3,
    });
    expect(anyRequest).not.toHaveProperty("hasJapaneseReadme");
    expect(anyRequest).not.toHaveProperty("page");

    expect(
      toRepositoryDiscoveryRequest({
        ...createDefaultRepositoryFilters(),
        hasJapaneseReadme: "no",
      }).hasJapaneseReadme,
    ).toBe(false);
  });

  it("normalizes equivalent lists for stable query identity", () => {
    const normalized = normalizeRepositoryFilters({
      ...createDefaultRepositoryFilters(),
      languages: ["typescript", " Go ", "TypeScript"],
    });
    expect(normalized.languages).toEqual(["Go", "typescript"]);
  });
});
