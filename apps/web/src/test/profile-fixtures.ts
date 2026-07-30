import type {
  GitHubUserEnvelope,
  ProfileAnalysisEnvelope,
} from "../shared/api/generated";

const meta = {
  rateLimitRemaining: 4_992,
  requestId: "req_profile_test",
  timestamp: "2026-07-30T00:00:00Z",
};

export const gitHubUserFixture: GitHubUserEnvelope = {
  data: {
    avatarUrl: "https://avatars.githubusercontent.com/u/1?v=4",
    bio: "Builds useful developer tools.",
    followers: 1_250,
    following: 42,
    login: "octocat",
    name: "The Octocat",
    publicRepos: 8,
    repositories: [
      {
        defaultBranch: "main",
        description: "A typed service",
        forks: 3,
        fullName: "octocat/typed-service",
        isArchived: false,
        isFork: false,
        mainLanguage: "TypeScript",
        name: "typed-service",
        openIssues: 4,
        owner: "octocat",
        pushedAt: "2026-07-29T00:00:00Z",
        stars: 120,
        updatedAt: "2026-07-29T00:00:00Z",
        url: "https://github.com/octocat/typed-service",
      },
    ],
  },
  meta,
};

export const profileAnalysisFixture: ProfileAnalysisEnvelope = {
  data: {
    frameworks: ["React", "Gin"],
    languages: [
      { name: "TypeScript", percentage: 65 },
      { name: "Go", percentage: 35 },
    ],
    repositoriesAnalyzed: 8,
    username: "octocat",
    warnings: [],
  },
  meta,
};

export function errorEnvelope(status: 404 | 429) {
  return {
    error: {
      code:
        status === 404 ? "GITHUB_USER_NOT_FOUND" : "GITHUB_RATE_LIMIT_EXCEEDED",
      message: status === 404 ? "GitHub user was not found" : "Rate limited",
    },
    meta,
  };
}
