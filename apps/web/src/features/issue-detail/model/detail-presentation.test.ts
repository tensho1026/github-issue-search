import { describe, expect, it } from "vitest";

import {
  aggregateStatusLabel,
  categoryLabel,
  qualitySignalLabel,
  repositorySignalLabel,
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
});
