import type { RepositoryDiscoveryEnvelope } from "../shared/api/generated";

export const repositoryDiscoveryFixture: RepositoryDiscoveryEnvelope = {
  data: {
    items: [
      {
        activity: {
          pushedAt: "2026-07-30T08:00:00Z",
          updatedAt: "2026-07-30T09:00:00Z",
        },
        category: "tooling",
        difficulty: {
          label: "very_low",
          level: 1,
          reasons: [
            "Contributing guide is available.",
            "Good first issues are available.",
          ],
        },
        documentation: {
          codeOfConduct: true,
          contributingGuide: true,
          japaneseReadme: {
            analyzedBytes: 4096,
            confidence: "medium",
            detected: true,
            japaneseRunes: 80,
            letterRunes: 200,
            status: "sampled",
          },
          readmeAvailable: true,
          securityPolicy: true,
          status: "sampled",
        },
        language: "TypeScript",
        license: {
          name: "MIT License",
          spdxId: "MIT",
          status: "exact",
        },
        popularity: {
          forks: 32,
          openIssues: 14,
          stars: 420,
          watchers: 18,
        },
        readiness: {
          band: "ready",
          discussionsEnabled: true,
          goodFirstIssues: 4,
          helpWantedIssues: 6,
          issuesEnabled: true,
          reasons: [
            "README is available.",
            "Contributing guide is available.",
            "Good first issues are available.",
          ],
          score: 88,
        },
        repository: {
          description: "A typed React and Go developer tool",
          fullName: "example/typed-service",
          isArchived: false,
          isFork: false,
          name: "typed-service",
          owner: "example",
          url: "https://github.com/example/typed-service",
        },
        technologies: ["React"],
        topics: ["developer-tools", "react"],
        warnings: [
          {
            code: "readme_content_sampled",
            message:
              "README language and technology evidence uses a bounded content sample.",
          },
        ],
      },
    ],
    pagination: {
      hasNext: false,
      page: 1,
      perPage: 20,
      total: 1,
      totalPages: 1,
    },
    searchSummary: {
      candidatesChecked: 50,
      enrichmentAttempted: 10,
      enrichmentFailed: 1,
      enrichmentIncomplete: true,
      githubIncomplete: true,
      upstreamTotal: 200,
    },
    warnings: [
      {
        code: "github_results_incomplete",
        message:
          "GitHub reported more repositories than the bounded candidate window.",
      },
      {
        code: "repository_enrichment_incomplete",
        message: "Some shortlist documentation evidence was unavailable.",
      },
    ],
  },
  meta: {
    rateLimitRemaining: 58,
    requestId: "req_repository_discovery",
    timestamp: "2026-07-30T10:00:00Z",
  },
};
