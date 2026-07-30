import type {
  AggregateStatus,
  IssueCategory,
  QualitySignal,
  RepositorySignal,
  ScoreComponent,
  SignalState,
} from "../../../shared/api/generated";

type BadgeTone = "danger" | "info" | "neutral" | "success" | "warning";

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
