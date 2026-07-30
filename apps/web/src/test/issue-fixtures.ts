import type { IssueSearchEnvelope } from "../shared/api/generated";

const evidence = {
  description: "Repository language matches the contributor profile.",
  ruleId: "skill.repository-language",
  source: "repository_language" as const,
};

export const issueSearchFixture: IssueSearchEnvelope = {
  data: {
    items: [
      {
        difficulty: {
          confidence: "high",
          label: "Intermediate",
          level: 3,
        },
        effort: {
          band: "half_day",
          confidence: "medium",
          label: "Half a day",
        },
        issue: {
          comments: 4,
          createdAt: "2026-07-01T00:00:00Z",
          estimatedDifficulty: 3,
          labels: ["good first issue", "accessibility"],
          number: 42,
          title: "Improve keyboard navigation in the command palette",
          updatedAt: "2026-07-29T00:00:00Z",
          url: "https://github.com/octocat/typed-service/issues/42",
        },
        recommendation: {
          breakdown: [
            {
              maximum: 30,
              name: "skill_match",
              reasons: ["TypeScript is a profile skill."],
              score: 25,
            },
            {
              maximum: 20,
              name: "issue_quality",
              reasons: ["The issue has acceptance criteria."],
              score: 18,
            },
            {
              maximum: 15,
              name: "activity",
              reasons: ["The repository is active."],
              score: 12,
            },
            {
              maximum: 15,
              name: "maintainer_responsiveness",
              reasons: ["Maintainers respond to contributors."],
              score: 10,
            },
            {
              maximum: 10,
              name: "repository_quality",
              reasons: ["CI and contribution docs are present."],
              score: 8,
            },
            {
              maximum: 10,
              name: "availability",
              reasons: ["The issue is open and unassigned."],
              score: 10,
            },
          ],
          claim: {
            claimed: false,
            confidence: "high",
            evidence: [],
          },
          reasons: [
            "The primary language matches your profile.",
            "The issue has clear acceptance criteria.",
          ],
          score: 83,
          skillMatch: {
            denominator: 2,
            matched: 1,
            percentage: 50,
            skills: [
              {
                evidence: [evidence],
                status: "matched",
                technology: "TypeScript",
              },
              {
                evidence: [],
                status: "unknown",
                technology: "Accessibility",
              },
            ],
          },
          warnings: [
            {
              code: "maintainer_sample_partial",
              evidence: [],
              message: "Maintainer response evidence is partial.",
              severity: "warning",
            },
          ],
        },
        repository: {
          description: "A typed service",
          fullName: "octocat/typed-service",
          isArchived: false,
          lastUpdatedAt: "2026-07-29T00:00:00Z",
          mainLanguage: "TypeScript",
          name: "typed-service",
          owner: "octocat",
          stars: 1250,
          url: "https://github.com/octocat/typed-service",
        },
      },
    ],
    pagination: {
      hasNext: true,
      page: 1,
      perPage: 20,
      total: 21,
      totalPages: 2,
    },
    searchSummary: {
      candidatesChecked: 50,
      enrichmentAttempted: 20,
      enrichmentFailed: 1,
      excludedByReason: [
        {
          count: 12,
          reason: "below_minimum_stars",
        },
      ],
      upstreamTotal: 1200,
    },
    warnings: [
      {
        code: "issue_enrichment_incomplete",
        message: "One repository used candidate metadata.",
      },
    ],
  },
  meta: {
    rateLimitRemaining: 4920,
    requestId: "req_issue_search",
    timestamp: "2026-07-30T00:00:00Z",
  },
};
