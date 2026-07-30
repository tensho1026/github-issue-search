import { describe, expect, it } from "vitest";

import { ApiError } from "../../../shared/api/client";
import {
  categoryLabel,
  confidenceLabel,
  difficultyPresentation,
  evidencePresentation,
  readinessPresentation,
  repositoryErrorPresentation,
} from "./repository-presentation";

describe("repository discovery presentation", () => {
  it("maps server evidence without recomputing decisions", () => {
    expect(categoryLabel("infrastructure")).toBe("Infrastructure");
    expect(confidenceLabel("medium")).toBe("Medium confidence");
    expect(evidencePresentation("sampled")).toEqual({
      label: "Sampled evidence",
      tone: "warning",
    });
    expect(
      readinessPresentation({
        band: "ready",
        discussionsEnabled: true,
        goodFirstIssues: 2,
        helpWantedIssues: 1,
        issuesEnabled: true,
        reasons: ["Starter issues are available."],
        score: 80,
      }),
    ).toMatchObject({ label: "Contribution ready", tone: "success" });
    expect(
      difficultyPresentation({
        label: "very_high",
        level: 5,
        reasons: ["No starter issue evidence."],
      }),
    ).toEqual({ label: "Very High", tone: "danger" });
  });

  it.each([
    [400, "Repository filters were rejected", false],
    [429, "GitHub needs a breather", false],
    [502, "GitHub discovery is unavailable", true],
    [504, "Discovery took too long", true],
  ] as const)("maps API status %d", (status, title, retryable) => {
    expect(
      repositoryErrorPresentation(
        new ApiError({
          code: "TEST",
          message: "test",
          requestId: "req_test",
          status,
        }),
      ),
    ).toMatchObject({ requestId: "req_test", retryable, title });
  });
});
