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
};

type DetailErrorDefinition = readonly [title: string, retryable: boolean];

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
  400: ["Issue reference rejected", false],
  404: ["Issue not found", false],
  429: ["GitHub needs a breather", false],
  502: ["GitHub detail is unavailable", true],
  504: ["Recommendation took too long", true],
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
        "An unexpected client error interrupted this recommendation. Retry the validated issue.",
      retryable: true,
      title: "Recommendation interrupted",
    };
  }

  const definition = detailErrors[error.status] ?? [
    "Recommendation unavailable",
    true,
  ];
  const [title, retryable] = definition;
  const description =
    error.status === 404
      ? "GitHub could not find this public issue. It may have moved or become private."
      : error.status === 429
        ? "The GitHub allowance is exhausted. Keep this URL and return later."
        : retryable
          ? "GitHub could not complete this recommendation. Retry the same URL."
          : "Return to search and open a fresh result.";
  const shared = error.requestId ? { requestId: error.requestId } : {};
  return { ...shared, description, retryable, title };
}
