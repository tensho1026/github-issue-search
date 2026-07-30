import { describe, expect, it } from "vitest";

import {
  formatCompactNumber,
  formatDate,
  formatDuration,
  formatPercentage,
} from "./format";

describe("display formatters", () => {
  it("bounds percentages", () => {
    expect(formatPercentage(-1)).toBe("0%");
    expect(formatPercentage(68.6)).toBe("69%");
    expect(formatPercentage(101)).toBe("100%");
  });

  it("formats public counts compactly", () => {
    expect(formatCompactNumber(1_250)).toMatch(/1\.3K/i);
    expect(formatCompactNumber(-1)).toBe("0");
  });

  it("keeps malformed upstream dates understandable", () => {
    expect(formatDate("not-a-date")).toBe("Unknown");
    expect(formatDate("2026-07-30T00:00:00Z")).toMatch(/30 Jul 2026/);
  });

  it.each([
    [null, "Unavailable"],
    [-1, "Unavailable"],
    [30, "< 1 min"],
    [120, "2 min"],
    [3600, "1 hr"],
    [7200, "2 hrs"],
    [172_800, "2 days"],
  ])("formats the duration %s as %s", (seconds, expected) => {
    expect(formatDuration(seconds)).toBe(expected);
  });
});
