import { describe, expect, it } from "vitest";

import { ApiError } from "../../../shared/api/client";
import {
  scorePresentation,
  searchErrorPresentation,
  skillPresentation,
  warningPresentation,
} from "./search-presentation";

describe("issue search presentation", () => {
  it.each([
    [88, "Strong fit"],
    [62, "Promising fit"],
    [42, "Explore carefully"],
  ])("maps score %d to one shared semantic label", (score, label) => {
    expect(scorePresentation(score).label).toBe(label);
  });

  it("maps repeated skill and warning states centrally", () => {
    expect(skillPresentation("matched")).toBe("success");
    expect(skillPresentation("unmatched")).toBe("warning");
    expect(skillPresentation("unknown")).toBe("neutral");
    expect(warningPresentation("critical")).toBe("danger");
    expect(warningPresentation("warning")).toBe("warning");
    expect(warningPresentation("info")).toBe("info");
  });

  it.each([
    [404, "Profile not found", false],
    [429, "GitHub needs a breather", false],
    [502, "GitHub search is unavailable", true],
    [504, "Search took too long", true],
  ] as const)(
    "maps API status %d to focused recovery guidance",
    (status, title, retryable) => {
      expect(
        searchErrorPresentation(
          new ApiError({
            code: "UPSTREAM_ERROR",
            message: "raw transport message",
            requestId: "req_search",
            status,
          }),
        ),
      ).toMatchObject({ requestId: "req_search", retryable, title });
    },
  );
});
