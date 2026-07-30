import { describe, expect, it } from "vitest";

import {
  decodeIssueDetailContext,
  issueDetailSearchParameters,
  validateIssueReference,
} from "./issue-reference";

describe("validateIssueReference", () => {
  it("normalizes a valid route reference", () => {
    expect(validateIssueReference(" OpenAI ", "openai-go", "42")).toEqual({
      issueNumber: 42,
      owner: "OpenAI",
      repository: "openai-go",
      valid: true,
    });
  });

  it.each([
    ["bad--owner", "repo", "1", /owner/],
    ["openai", "..", "1", /repository/],
    ["openai", "repo", "0", /positive integer/],
    ["openai", "repo", "9007199254740992", /supported range/],
  ])(
    "rejects the invalid route %s/%s/%s",
    (owner, repository, issueNumber, message) => {
      const result = validateIssueReference(owner, repository, issueNumber);
      expect(result.valid).toBe(false);
      if (!result.valid) {
        expect(result.message).toMatch(message);
      }
    },
  );
});

describe("issue detail search context", () => {
  const searchLocation =
    "/search?username=octocat&language=TypeScript&framework=React&label=bug&minimumStars=10&maximumDifficulty=3&updatedWithinDays=180&includeDocumentation=false&includeEnglish=true&excludeArchived=true&page=2&perPage=20&search=1";

  it("preserves a bounded search location for a detail link", () => {
    expect(issueDetailSearchParameters(searchLocation).get("from")).toBe(
      searchLocation,
    );
    expect(issueDetailSearchParameters("/profiles/octocat").size).toBe(0);
  });

  it("derives skills and a canonical return location", () => {
    const result = decodeIssueDetailContext(
      issueDetailSearchParameters(searchLocation),
    );

    expect(result.valid).toBe(true);
    if (result.valid) {
      expect(result.returnTo).toContain("/search?username=octocat");
      expect(result.skills).toEqual(["TypeScript", "React"]);
    }
  });

  it.each([
    "https://example.com/search?search=1",
    "//example.com/search?search=1",
    "/profiles/octocat?search=1",
    "/search?username=bad--name&search=1",
    "/search?username=octocat&search=0",
  ])("rejects an unsafe or invalid return location: %s", (from) => {
    const result = decodeIssueDetailContext(new URLSearchParams({ from }));

    expect(result.valid).toBe(false);
  });

  it("rejects duplicate return locations", () => {
    const parameters = new URLSearchParams();
    parameters.append("from", searchLocation);
    parameters.append("from", searchLocation);

    expect(decodeIssueDetailContext(parameters).valid).toBe(false);
  });
});
