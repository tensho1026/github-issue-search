import { ApiError } from "../../../shared/api/client";

export type ProfileErrorPresentation = {
  description: string;
  requestId?: string;
  retryable: boolean;
  title: string;
  tone: "danger" | "warning";
};

export function profileErrorPresentation(
  error: Error,
  username: string,
): ProfileErrorPresentation {
  if (error instanceof ApiError) {
    switch (error.status) {
      case 400:
        return {
          description:
            "That username does not match GitHub’s public login rules. Check it and try again.",
          requestId: error.requestId,
          retryable: false,
          title: "This username needs a second look",
          tone: "warning",
        };
      case 404:
        return {
          description: `We could not find a public GitHub profile for @${username}. It may have been renamed or removed.`,
          requestId: error.requestId,
          retryable: false,
          title: "Profile not found",
          tone: "warning",
        };
      case 429:
        return {
          description:
            "GitHub’s public API limit is temporarily exhausted. Nothing is lost—wait a moment, then retry.",
          requestId: error.requestId,
          retryable: true,
          title: "GitHub needs a breather",
          tone: "warning",
        };
      default:
        return {
          description:
            "The profile signal could not be completed. Retry the bounded public analysis.",
          requestId: error.requestId,
          retryable: true,
          title: "The signal dropped",
          tone: "danger",
        };
    }
  }
  return {
    description:
      "A network interruption stopped the analysis. Check your connection and retry.",
    retryable: true,
    title: "Connection interrupted",
    tone: "danger",
  };
}

export function prioritizedProfileError(
  errors: ReadonlyArray<Error | null>,
): Error | null {
  const present = errors.filter((error): error is Error => error !== null);
  return (
    present.find(
      (error) => error instanceof ApiError && error.status === 429,
    ) ??
    present.find(
      (error) => error instanceof ApiError && error.status === 404,
    ) ??
    present[0] ??
    null
  );
}
