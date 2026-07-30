import { ApiError } from "../../../shared/api/client";
import type {
  RecommendationWarning,
  SkillMatchItem,
} from "../../../shared/api/generated";

export type SearchErrorPresentation = {
  description: string;
  requestId?: string;
  retryable: boolean;
  title: string;
  tone: "danger" | "warning";
};

export function scorePresentation(score: number): {
  className: string;
  label: string;
} {
  if (score >= 75) {
    return {
      className: "border-score-high/35 bg-success-soft text-score-high",
      label: "Strong fit",
    };
  }
  if (score >= 50) {
    return {
      className: "border-score-medium/35 bg-warning-soft text-score-medium",
      label: "Promising fit",
    };
  }
  return {
    className: "border-score-low/35 bg-danger-soft text-score-low",
    label: "Explore carefully",
  };
}

export function skillPresentation(
  status: SkillMatchItem["status"],
): "neutral" | "success" | "warning" {
  switch (status) {
    case "matched":
      return "success";
    case "unmatched":
      return "warning";
    case "unknown":
      return "neutral";
  }
}

export function warningPresentation(
  severity: RecommendationWarning["severity"],
): "danger" | "info" | "warning" {
  switch (severity) {
    case "critical":
      return "danger";
    case "warning":
      return "warning";
    case "info":
      return "info";
  }
}

export function searchErrorPresentation(error: Error): SearchErrorPresentation {
  if (!(error instanceof ApiError)) {
    return {
      description:
        "An unexpected client error interrupted the search. Retry the same validated filters.",
      retryable: true,
      title: "Search was interrupted",
      tone: "danger",
    };
  }

  const shared = error.requestId ? { requestId: error.requestId } : {};
  switch (error.status) {
    case 400:
      return {
        ...shared,
        description:
          "The API rejected these filters. Review the editable form and submit a new search.",
        retryable: false,
        title: "Search filters were rejected",
        tone: "danger",
      };
    case 403:
      return {
        ...shared,
        description:
          "This application origin is not permitted by the API configuration.",
        retryable: false,
        title: "Search is not permitted",
        tone: "danger",
      };
    case 404:
      return {
        ...shared,
        description:
          "GitHub could not find that public profile. Check the username and try another search.",
        retryable: false,
        title: "Profile not found",
        tone: "warning",
      };
    case 429:
      return {
        ...shared,
        description:
          "The bounded GitHub allowance is exhausted for now. Keep this URL and return after the limit resets.",
        retryable: false,
        title: "GitHub needs a breather",
        tone: "warning",
      };
    case 502:
      return {
        ...shared,
        description:
          "GitHub returned an incomplete or unavailable search response. Your URL and filters are still safe.",
        retryable: true,
        title: "GitHub search is unavailable",
        tone: "warning",
      };
    case 504:
      return {
        ...shared,
        description:
          "The bounded upstream request exceeded its deadline. Retry without changing the shareable filters.",
        retryable: true,
        title: "Search took too long",
        tone: "warning",
      };
    default:
      return {
        ...shared,
        description:
          "The API could not complete this search. Your URL and filter choices remain available.",
        retryable: true,
        title: "Search could not be completed",
        tone: "danger",
      };
  }
}
