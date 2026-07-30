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
    analysisWindow: {
      days: 365,
      from: "2025-07-30T00:00:00Z",
      publicOnly: true,
      to: "2026-07-30T00:00:00Z",
    },
    contributions: {
      commits: { status: "sampled", value: 18 },
      issuesOpened: { status: "exact", value: 3 },
      pullRequestReviews: { status: "sampled", value: 4 },
      pullRequestsOpened: { status: "exact", value: 7 },
      repositoriesTouched: { status: "sampled", value: 1 },
      windowDays: 365,
    },
    frameworks: ["React", "Gin"],
    languages: [
      { name: "TypeScript", percentage: 65 },
      { name: "Go", percentage: 35 },
    ],
    ossExperience: {
      confidence: "high",
      evidence: [
        {
          kind: "authored_pull_requests",
          status: "exact",
          value: 7,
        },
      ],
      level: "active",
      publicOnly: true,
    },
    proficiency: [
      {
        confidence: "medium",
        evidence: [
          {
            kind: "language_share_percentage",
            status: "exact",
            value: 65,
          },
        ],
        kind: "language",
        label: "developing",
        level: 2,
        name: "TypeScript",
        score: 39,
      },
    ],
    recentTechnologies: [
      {
        confidence: "medium",
        kind: "language",
        lastUsedAt: "2026-07-29T00:00:00Z",
        name: "TypeScript",
        repositoryCount: 2,
        repositorySources: ["contributed", "owned"],
      },
    ],
    repositoryEvidence: {
      contributed: {
        activeInWindow: 1,
        limit: 20,
        observed: 1,
        primaryTechnologies: [{ name: "TypeScript", percentage: 100 }],
        status: "exact",
        total: 1,
      },
      forked: {
        activeInWindow: 1,
        limit: 20,
        observed: 1,
        primaryTechnologies: [{ name: "Go", percentage: 100 }],
        status: "exact",
        total: 1,
      },
      owned: {
        activeInWindow: 8,
        limit: 20,
        observed: 8,
        primaryTechnologies: [
          { name: "TypeScript", percentage: 65 },
          { name: "Go", percentage: 35 },
        ],
        status: "exact",
        total: 8,
      },
      starred: {
        activeInWindow: 1,
        limit: 20,
        observed: 1,
        primaryTechnologies: [{ name: "Rust", percentage: 100 }],
        status: "sampled",
        total: null,
      },
    },
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
