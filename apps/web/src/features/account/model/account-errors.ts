import { ApiError } from "../../../shared/api/client";

export type AccountErrorPresentation = {
  description: string;
  title: string;
  variant: "danger" | "warning";
};

export function accountErrorPresentation(
  error: unknown,
): AccountErrorPresentation {
  if (error instanceof ApiError) {
    if (error.status === 401) {
      return {
        description:
          "Your secure session expired. Unsaved fields remain on this page; sign in again to submit them.",
        title: "Session expired",
        variant: "warning",
      };
    }
    if (error.status === 409) {
      return {
        description:
          "This item changed in another tab. Reload the latest account data and try again.",
        title: "Newer account data exists",
        variant: "warning",
      };
    }
    if (error.status === 503) {
      return {
        description:
          "Account storage is temporarily unavailable. Public searches and analysis still work without it.",
        title: "Account storage unavailable",
        variant: "warning",
      };
    }
    if (error.status === 400) {
      return {
        description:
          "The account service rejected an invalid value. Review the form and try again.",
        title: "Check the submitted values",
        variant: "danger",
      };
    }
  }
  return {
    description:
      "The request could not be confirmed. Your public work and unsaved fields were preserved.",
    title: "Account request failed",
    variant: "danger",
  };
}
