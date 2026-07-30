import { describe, expect, it } from "vitest";

import type {
  GitHubUser,
  ProfileAnalysis,
} from "../../../shared/api/generated";
import {
  featuredRepositories,
  profileTechnologyTags,
  sortLanguages,
} from "./profile-view";

const analysis: ProfileAnalysis = {
  frameworks: ["React", "react", "Gin"],
  languages: [
    { name: "TypeScript", percentage: 70 },
    { name: "Go", percentage: 30 },
  ],
  repositoriesAnalyzed: 2,
  username: "octocat",
  warnings: [],
};

describe("profile view selectors", () => {
  it("sorts language copies without mutating contract data", () => {
    const sorted = sortLanguages(analysis.languages, "alphabetical");
    expect(sorted.map(({ name }) => name)).toEqual(["Go", "TypeScript"]);
    expect(analysis.languages[0]?.name).toBe("TypeScript");
  });

  it("deduplicates technology tags case-insensitively", () => {
    expect(profileTechnologyTags(analysis)).toEqual([
      "Gin",
      "Go",
      "React",
      "TypeScript",
    ]);
  });

  it("selects featured repositories deterministically", () => {
    const user = {
      repositories: [
        { fullName: "octocat/b", stars: 10 },
        { fullName: "octocat/a", stars: 10 },
        { fullName: "octocat/c", stars: 20 },
      ],
    } as GitHubUser;
    expect(
      featuredRepositories(user, 2).map(({ fullName }) => fullName),
    ).toEqual(["octocat/c", "octocat/a"]);
  });
});
