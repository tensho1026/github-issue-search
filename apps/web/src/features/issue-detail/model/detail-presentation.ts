import type {
  AggregateStatus,
  ChangeScope,
  IssueCategory,
  QualitySignal,
  RepositorySignal,
  ScoreComponent,
  SignalState,
} from "../../../shared/api/generated";
import { ApiError } from "../../../shared/api/client";

type BadgeTone = "danger" | "info" | "neutral" | "success" | "warning";

export type DetailErrorPresentation = {
  description: string;
  requestId?: string;
  retryable: boolean;
  title: string;
  tone: "danger" | "warning";
};

type DetailErrorDefinition = readonly [
  title: string,
  description: string,
  retryable: boolean,
  tone: DetailErrorPresentation["tone"],
];

const qualityLabels: Record<QualitySignal["key"], string> = {
  acceptance_criteria: "Acceptance criteria",
  current_behavior: "Current behavior",
  expected_behavior: "Expected behavior",
  implementation_guidance: "Implementation guidance",
  problem_description: "Problem description",
  related_files: "Related files",
  reproduction_steps: "Reproduction steps",
  screenshot: "Screenshot or visual",
  test_method: "Test method",
};

const repositorySignalLabels: Record<RepositorySignal["key"], string> = {
  ci: "Continuous integration",
  code_of_conduct: "Code of conduct",
  contributing: "Contributing guide",
  readme: "README",
  tests: "Automated tests",
};

const scoreLabels: Record<ScoreComponent["name"], string> = {
  activity: "Recent activity",
  availability: "Availability",
  issue_quality: "Issue quality",
  maintainer_responsiveness: "Maintainer response",
  repository_quality: "Repository readiness",
  skill_match: "Skill match",
};

const scopeAreaLabels: Record<ChangeScope["areas"][number], string> = {
  backend: "Backend",
  documentation: "Documentation",
  frontend: "Frontend",
  infrastructure: "Infrastructure",
  migration: "Migration",
  tests: "Tests",
};

const detailErrors: Record<number, DetailErrorDefinition> = {
  400: [
    "Issue reference rejected",
    "The API rejected this issue or skill context. Open a fresh search result.",
    false,
    "danger",
  ],
  404: [
    "Issue not found",
    "GitHub could not find this public issue. It may have been removed, transferred, or made private.",
    false,
    "warning",
  ],
  429: [
    "GitHub needs a breather",
    "The GitHub allowance is exhausted. Keep this URL and return later.",
    false,
    "warning",
  ],
  502: [
    "GitHub detail is unavailable",
    "GitHub returned an incomplete detail snapshot. The issue URL remains safe.",
    true,
    "warning",
  ],
  504: [
    "Recommendation took too long",
    "The upstream request timed out. Retry without changing this URL.",
    true,
    "warning",
  ],
};

export function categoryLabel(category: IssueCategory): string {
  return category === "ui"
    ? "UI"
    : category === "devops"
      ? "DevOps"
      : category.charAt(0).toUpperCase() + category.slice(1);
}

export function qualitySignalLabel(key: QualitySignal["key"]): string {
  return qualityLabels[key];
}

export function repositorySignalLabel(key: RepositorySignal["key"]): string {
  return repositorySignalLabels[key];
}

export function scoreComponentLabel(name: ScoreComponent["name"]): string {
  return scoreLabels[name];
}

export function scopeAreaLabel(area: ChangeScope["areas"][number]): string {
  return scopeAreaLabels[area];
}

export function signalPresentation(state: SignalState): {
  label: string;
  tone: BadgeTone;
} {
  switch (state) {
    case "present":
      return { label: "Present", tone: "success" };
    case "absent":
      return { label: "Not found", tone: "warning" };
    case "not_applicable":
      return { label: "Not applicable", tone: "neutral" };
    case "unknown":
      return { label: "Unknown", tone: "neutral" };
  }
}

export function aggregateStatusLabel(status: AggregateStatus): string {
  return status === "available" ? "Available" : "Unavailable";
}

export function detailErrorPresentation(error: Error): DetailErrorPresentation {
  if (!(error instanceof ApiError)) {
    return {
      description:
        "An unexpected client error interrupted this recommendation. You can retry the same validated issue.",
      retryable: true,
      title: "Recommendation interrupted",
      tone: "danger",
    };
  }

  const definition = detailErrors[error.status] ?? [
    "Recommendation unavailable",
    "The API could not complete this recommendation. Your links remain available.",
    true,
    "danger",
  ];
  const [title, description, retryable, tone] = definition;
  const shared = error.requestId ? { requestId: error.requestId } : {};
  return { ...shared, description, retryable, title, tone };
}
