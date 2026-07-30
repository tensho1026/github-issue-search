import { ApiError } from "../../../shared/api/client";
import type {
  EvidenceConfidence,
  EvidenceStatus,
  OssCategory,
  RepositoryDiscoveryDifficulty,
  RepositoryDiscoveryReadiness,
} from "../../../shared/api/generated";

export type RepositoryErrorPresentation = {
  description: string;
  requestId?: string;
  retryable: boolean;
  title: string;
  tone: "danger" | "warning";
};

export function categoryLabel(category: OssCategory): string {
  return category.charAt(0).toUpperCase() + category.slice(1);
}

export function enumLabel(value: string): string {
  return value
    .split("_")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function evidencePresentation(status: EvidenceStatus): {
  label: string;
  tone: "neutral" | "success" | "warning";
} {
  switch (status) {
    case "exact":
      return { label: "Exact evidence", tone: "success" };
    case "sampled":
      return { label: "Sampled evidence", tone: "warning" };
    case "unavailable":
      return { label: "Evidence unavailable", tone: "neutral" };
  }
}

export function confidenceLabel(confidence: EvidenceConfidence): string {
  return `${enumLabel(confidence)} confidence`;
}

export function readinessPresentation(
  readiness: RepositoryDiscoveryReadiness,
): {
  label: string;
  tone: "neutral" | "success" | "warning";
} {
  switch (readiness.band) {
    case "ready":
      return { label: "Contribution ready", tone: "success" };
    case "promising":
      return { label: "Promising readiness", tone: "warning" };
    case "needs_work":
      return { label: "Needs preparation", tone: "neutral" };
  }
}

export function difficultyPresentation(
  difficulty: RepositoryDiscoveryDifficulty,
): {
  label: string;
  tone: "danger" | "neutral" | "success" | "warning";
} {
  if (difficulty.level <= 2) {
    return { label: enumLabel(difficulty.label), tone: "success" };
  }
  if (difficulty.level === 3) {
    return { label: enumLabel(difficulty.label), tone: "warning" };
  }
  return { label: enumLabel(difficulty.label), tone: "danger" };
}

export function repositoryErrorPresentation(
  error: Error,
): RepositoryErrorPresentation {
  if (!(error instanceof ApiError)) {
    return {
      description:
        "An unexpected client error interrupted discovery. Retry the same validated URL.",
      retryable: true,
      title: "Discovery was interrupted",
      tone: "danger",
    };
  }
  const shared = error.requestId ? { requestId: error.requestId } : {};
  switch (error.status) {
    case 400:
      return {
        ...shared,
        description:
          "The API rejected these filters. Review the editable controls and submit a new search.",
        retryable: false,
        title: "Repository filters were rejected",
        tone: "danger",
      };
    case 403:
      return {
        ...shared,
        description:
          "This application origin is not permitted by the API configuration.",
        retryable: false,
        title: "Discovery is not permitted",
        tone: "danger",
      };
    case 429:
      return {
        ...shared,
        description:
          "The bounded GitHub allowance is exhausted. Keep this URL and return after the limit resets.",
        retryable: false,
        title: "GitHub needs a breather",
        tone: "warning",
      };
    case 502:
      return {
        ...shared,
        description:
          "GitHub could not provide the required public repository window. Your URL and filters remain safe.",
        retryable: true,
        title: "GitHub discovery is unavailable",
        tone: "warning",
      };
    case 504:
      return {
        ...shared,
        description:
          "The bounded GitHub request exceeded its deadline. Retry without changing the shareable filters.",
        retryable: true,
        title: "Discovery took too long",
        tone: "warning",
      };
    default:
      return {
        ...shared,
        description:
          "The API could not complete repository discovery. No anonymous search data was persisted.",
        retryable: true,
        title: "Discovery could not be completed",
        tone: "danger",
      };
  }
}
