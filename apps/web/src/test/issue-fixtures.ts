import type {
  IssueDetailEnvelope,
  IssueSearchEnvelope,
} from "../shared/api/generated";

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

export const issueDetailFixture: IssueDetailEnvelope = {
  data: {
    activity: {
      ci: "success",
      contributors: {
        confidence: "high",
        sampleSize: 24,
        status: "available",
        truncated: false,
        value: 8,
        windowDays: 90,
      },
      issueResponse: {
        confidence: "medium",
        medianSeconds: 7200,
        percentile90Seconds: 86_400,
        sampleSize: 18,
        status: "available",
        truncated: false,
        windowDays: 90,
      },
      lastMeaningfulUpdate: "2026-07-29T00:00:00Z",
      pullRequestMerge: {
        confidence: "high",
        denominator: 20,
        numerator: 15,
        percentage: 75,
        sampleSize: 20,
        status: "available",
        truncated: false,
        windowDays: 90,
      },
      pullRequestMergeTime: {
        confidence: "medium",
        medianSeconds: 172_800,
        percentile90Seconds: 432_000,
        sampleSize: 15,
        status: "available",
        truncated: false,
        windowDays: 90,
      },
      pullRequestReview: {
        confidence: "medium",
        medianSeconds: 14_400,
        percentile90Seconds: 172_800,
        sampleSize: 16,
        status: "available",
        truncated: false,
        windowDays: 90,
      },
      pullRequestsOpened: {
        confidence: "high",
        sampleSize: 20,
        status: "available",
        truncated: false,
        value: 20,
        windowDays: 90,
      },
      staleOpenPullRequests: {
        confidence: "medium",
        sampleSize: 7,
        status: "available",
        truncated: false,
        value: 2,
        windowDays: 90,
      },
      unansweredIssues: {
        confidence: "medium",
        sampleSize: 28,
        status: "available",
        truncated: true,
        value: 3,
        windowDays: 90,
      },
    },
    analysis: {
      category: {
        confidence: "high",
        evidence: [
          {
            description: "The title describes keyboard accessibility.",
            ruleId: "category.accessibility",
            source: "title",
          },
        ],
        matches: ["accessibility", "ui", "testing"],
        primary: "accessibility",
      },
      confidence: "high",
      difficulty: {
        confidence: "high",
        evidence: [
          {
            description: "The change crosses UI behavior and tests.",
            ruleId: "difficulty.cross-cutting",
            source: "derived",
          },
        ],
        label: "Intermediate",
        level: 3,
      },
      effort: {
        band: "half_day",
        confidence: "medium",
        evidence: [
          {
            description: "The described scope fits a focused half-day change.",
            ruleId: "effort.scope",
            source: "derived",
          },
        ],
        label: "Half a day",
      },
      quality: {
        confidence: "high",
        score: 82,
        signals: [
          {
            evidence: [
              {
                description: "The body explains the keyboard trap.",
                ruleId: "quality.problem-description",
                source: "body",
              },
            ],
            key: "problem_description",
            state: "present",
          },
          {
            evidence: [],
            key: "current_behavior",
            state: "present",
          },
          {
            evidence: [],
            key: "expected_behavior",
            state: "present",
          },
          {
            evidence: [],
            key: "reproduction_steps",
            state: "present",
          },
          {
            evidence: [],
            key: "acceptance_criteria",
            state: "present",
          },
          {
            evidence: [],
            key: "implementation_guidance",
            state: "unknown",
          },
          {
            evidence: [],
            key: "related_files",
            state: "present",
          },
          {
            evidence: [],
            key: "test_method",
            state: "present",
          },
          {
            evidence: [],
            key: "screenshot",
            state: "not_applicable",
          },
        ],
      },
      requiredTechnologies: [
        {
          confidence: "high",
          evidence: [evidence],
          kind: "language",
          name: "TypeScript",
        },
        {
          confidence: "high",
          evidence: [
            {
              description: "The affected interface is built with React.",
              ruleId: "technology.react",
              source: "body",
            },
          ],
          kind: "framework",
          name: "React",
        },
        {
          confidence: "medium",
          evidence: [
            {
              description: "The issue requires keyboard interaction testing.",
              ruleId: "technology.accessibility",
              source: "derived",
            },
          ],
          kind: "practice",
          name: "Accessibility testing",
        },
      ],
      scope: {
        areas: ["frontend", "tests"],
        confidence: "high",
        databaseChange: "not_applicable",
        evidence: [
          {
            description: "The body names a UI component and its tests.",
            ruleId: "scope.frontend-tests",
            source: "body",
          },
        ],
        fileCount: {
          label: "3–5 files",
          maximum: 5,
          minimum: 3,
        },
      },
    },
    inspection: {
      incomplete: false,
    },
    issue: {
      assignees: [],
      author: {
        login: "hubot",
        type: "User",
      },
      body: [
        "## Current behavior",
        "Tab focus can become trapped inside the command palette.",
        "",
        "## Acceptance criteria",
        "- Escape closes the palette",
        "- Focus returns to the trigger",
        "- Add keyboard interaction tests",
      ].join("\n"),
      comments: 4,
      createdAt: "2026-07-01T00:00:00Z",
      labels: ["good first issue", "accessibility"],
      locked: false,
      number: 42,
      state: "open",
      title: "Improve keyboard navigation in the command palette",
      updatedAt: "2026-07-29T00:00:00Z",
      url: "https://github.com/octocat/typed-service/issues/42",
    },
    recommendation: issueSearchFixture.data.items[0]!.recommendation,
    repository: {
      defaultBranch: "main",
      description: "A typed service",
      forks: 84,
      fullName: "octocat/typed-service",
      id: 123_456,
      isArchived: false,
      isFork: false,
      mainLanguage: "TypeScript",
      name: "typed-service",
      openIssues: 32,
      owner: "octocat",
      pushedAt: "2026-07-29T00:00:00Z",
      stars: 1250,
      updatedAt: "2026-07-29T00:00:00Z",
      url: "https://github.com/octocat/typed-service",
    },
    repositoryHealth: [
      {
        evidence: [],
        key: "readme",
        state: "present",
      },
      {
        evidence: [],
        key: "contributing",
        state: "present",
      },
      {
        evidence: [],
        key: "code_of_conduct",
        state: "present",
      },
      {
        evidence: [],
        key: "ci",
        state: "present",
      },
      {
        evidence: [],
        key: "tests",
        state: "present",
      },
    ],
  },
  meta: {
    rateLimitRemaining: 4910,
    requestId: "req_issue_detail",
    timestamp: "2026-07-30T00:00:00Z",
  },
};
