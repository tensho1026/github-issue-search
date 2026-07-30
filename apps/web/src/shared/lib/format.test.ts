import { describe, expect, it } from "vitest";

import { formatCompactNumber, formatDate, formatPercentage } from "./format";

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
});
