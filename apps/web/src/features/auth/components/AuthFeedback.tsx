import { useEffect, useState } from "react";
import { useSearchParams } from "react-router";

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../../components/ui/alert";
import { useAuth } from "../auth-context";

type AuthMarker = "denied" | "error" | "success";

function readMarker(parameters: URLSearchParams): AuthMarker | undefined {
  const values = parameters.getAll("auth");
  if (values.length !== 1) {
    return undefined;
  }
  const value = values[0];
  return value === "denied" || value === "error" || value === "success"
    ? value
    : undefined;
}

export function AuthFeedback() {
  const [parameters, setParameters] = useSearchParams();
  const { query } = useAuth();
  const { refetch } = query;
  const [marker] = useState(() => readMarker(parameters));

  useEffect(() => {
    if (!parameters.has("auth")) {
      return;
    }
    const next = new URLSearchParams(parameters);
    next.delete("auth");
    setParameters(next, { replace: true });
  }, [parameters, setParameters]);

  useEffect(() => {
    if (marker === "success") {
      void refetch();
    }
  }, [marker, refetch]);

  if (!marker) {
    return null;
  }

  const content = {
    denied: {
      description:
        "GitHub authorization was cancelled. Your current page and anonymous work were preserved.",
      title: "Sign-in cancelled",
      variant: "info" as const,
    },
    error: {
      description:
        "GitHub sign-in could not be completed. Public IssueScout features remain available.",
      title: "Sign-in was not completed",
      variant: "danger" as const,
    },
    success: {
      description:
        "Your secure IssueScout session is ready. No GitHub token is stored in this browser.",
      title: "Signed in successfully",
      variant: "success" as const,
    },
  }[marker];

  return (
    <div className="mx-auto w-full max-w-7xl px-5 pt-4 sm:px-8 lg:px-10">
      <Alert variant={content.variant}>
        <AlertTitle>{content.title}</AlertTitle>
        <AlertDescription>{content.description}</AlertDescription>
      </Alert>
    </div>
  );
}
