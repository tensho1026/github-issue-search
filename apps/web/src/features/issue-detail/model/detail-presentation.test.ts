import { describe, expect, it } from "vitest";

import { ApiError } from "../../../shared/api/client";
import {
  aggregateStatusLabel,
  categoryLabel,
  detailErrorPresentation,
  qualitySignalLabel,
  repositorySignalLabel,
  scopeAreaLabel,
  scoreComponentLabel,
  signalPresentation,
} from "./detail-presentation";

describe("issue detail presentation", () => {
  it("provides stable human labels for contract keys", () => {
    expect(categoryLabel("devops")).toBe("DevOps");
    expect(categoryLabel("ui")).toBe("UI");
    expect(categoryLabel("accessibility")).toBe("Accessibility");
    expect(qualitySignalLabel("acceptance_criteria")).toBe(
      "Acceptance criteria",
    );
    expect(repositorySignalLabel("code_of_conduct")).toBe("Code of conduct");
    expect(scoreComponentLabel("maintainer_responsiveness")).toBe(
      "Maintainer response",
    );
    expect(scopeAreaLabel("infrastructure")).toBe("Infrastructure");
  });

  it("distinguishes missing, unknown, and inapplicable evidence", () => {
    expect(signalPresentation("present")).toEqual({
      label: "Present",
      tone: "success",
    });
    expect(signalPresentation("absent")).toEqual({
      label: "Not found",
      tone: "warning",
    });
    expect(signalPresentation("unknown")).toEqual({
      label: "Unknown",
      tone: "neutral",
    });
    expect(signalPresentation("not_applicable")).toEqual({
      label: "Not applicable",
      tone: "neutral",
    });
    expect(aggregateStatusLabel("unavailable")).toBe("Unavailable");
  });

  it("turns typed API failures into stable recovery guidance", () => {
    expect(
      detailErrorPresentation(
        new ApiError({
          code: "NOT_FOUND",
          message: "upstream",
          requestId: "req_detail",
          status: 404,
        }),
      ),
    ).toEqual({
      description:
        "GitHub could not find this public issue. It may have been removed, transferred, or made private.",
      requestId: "req_detail",
      retryable: false,
      title: "Issue not found",
      tone: "warning",
    });
    expect(detailErrorPresentation(new Error("network"))).toMatchObject({
      retryable: true,
      title: "Recommendation interrupted",
    });
  });
});
